/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"

	sourceController "github.com/krok-o/operator/pkg/source-controller"
)

var (
	finalizer                 = "event.krok.app/finalizer"
	outputSecretAnnotationKey = "output-to-secret"

	krokAnnotationValue = "krok-job"
	krokAnnotationKey   = "krok-app"
	ownerCommandName    = "command-name"
	dependenciesKey     = "job-dependencies"
	//outputKey           = "jobOutput"
	beginOutputFormat = "----- BEGIN OUTPUT -----"
	endOutputFormat   = "----- END OUTPUT -----"
)

// KrokEventReconciler reconciles a KrokEvent object
type KrokEventReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CommandTimeout    int
	PlatformProviders map[string]providers.Platform
	SourceController  *sourceController.Server
}

//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/finalizers,verbs=update
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokcommands,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/log,verbs=get
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *KrokEventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("event-controller")
	log.V(4).Info("starting reconcile loop")

	event := &v1alpha1.KrokEvent{}
	if err := r.Client.Get(ctx, req.NamespacedName, event); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get event object: %w", err)
	}

	log.Info("found event", "event", klog.KObj(event))

	if event.DeletionTimestamp != nil {
		return r.reconcileDelete(ctx, event)
	}

	repository := &v1alpha1.KrokRepository{}
	if err := GetParentObject(ctx, r.Client, "KrokRepository", v1alpha1.GroupVersion.Group, event, repository); err != nil {
		return ctrl.Result{
			RequeueAfter: 20 * time.Second,
		}, fmt.Errorf("failed to find parent for event: %w", err)
	}

	log.V(4).Info("found repository", "repository", klog.KObj(repository))
	newEvent := event.DeepCopy()
outer:
	for _, cmd := range newEvent.GetCommandsToRun() {
		if _, ok := newEvent.Status.RunningCommands[cmd.Name]; !ok {
			var listOfExtraInput []v1alpha1.Ref
			for _, dep := range cmd.Spec.Dependencies {
				// if the dependency is in the succeeded job list...
				// skip creation
				if !newEvent.Status.SucceededCommands.Has(dep) {
					continue outer
				}

				dependingCommand := &v1alpha1.KrokCommand{}
				if err := r.Get(ctx, types.NamespacedName{
					Name:      dep,
					Namespace: event.Namespace,
				}, dependingCommand); err != nil {
					return ctrl.Result{
						RequeueAfter: 10 * time.Second,
					}, err
				}
				// If the depending job defined output, we add those to the command.
				// TODO: Have a think about this. The job might have output which the depending command doesn't need.
				// Maybe then the command just has to define something like, +optional.
				if dependingCommand.Spec.CommandHasOutputToWrite {
					listOfExtraInput = append(listOfExtraInput, v1alpha1.Ref{
						Namespace: event.Namespace,
						Name:      r.generateJobName(event.Name, dependingCommand.Name),
					})
				}
			}

			// add any extra secrets which contain any outputs from different commands
			cmd.Spec.ReadInputFromSecrets = append(cmd.Spec.ReadInputFromSecrets, listOfExtraInput...)

			artifactURL, err := r.artifactURL(newEvent, repository)
			if err != nil {
				return ctrl.Result{
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("failed to checkout source: %w", err)
			}
			if err := r.createJob(ctx, log, &cmd, newEvent, repository, artifactURL); err != nil {
				return ctrl.Result{
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("failed to create jobs: %w", err)
			}

			if newEvent.Status.RunningCommands == nil {
				newEvent.Status.RunningCommands = make(map[string]bool)
			}
			newEvent.Status.RunningCommands[cmd.Name] = true
			continue
		}

		jobName := r.generateJobName(newEvent.Name, cmd.Name)
		job := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      jobName,
			Namespace: event.Namespace,
		}, job); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("failed to get job: %w", err)
		}

		switch {
		case r.HasCondition(job, batchv1.JobFailed):
			event.Status.FailedCommands = append(event.Status.FailedCommands, v1alpha1.Command{
				Name:    cmd.Name,
				Outcome: r.ConditionReason(job, batchv1.JobFailed),
			})
			if err := r.Delete(ctx, job); err != nil {
				return ctrl.Result{
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("failed to delete job: %w", err)
			}
			delete(event.Status.RunningCommands, cmd.Name)
		case r.HasCondition(job, batchv1.JobComplete):
			event.Status.SucceededCommands = append(event.Status.SucceededCommands, v1alpha1.Command{
				Name:    cmd.Name,
				Outcome: "success",
			})
			if err := r.Delete(ctx, job); err != nil {
				return ctrl.Result{
					RequeueAfter: 30 * time.Second,
				}, fmt.Errorf("failed to delete job: %w", err)
			}
			delete(event.Status.RunningCommands, cmd.Name)
		}
	}

	// Patch the event with updated status.
	if err := patchObject(ctx, r.Client, event, newEvent); err != nil {
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, fmt.Errorf("failed to patch object: %w", err)
	}

	// Done for now. This event will be re-triggered if one of its owning jobs is updated.
	return ctrl.Result{}, nil
}

func (r *KrokEventReconciler) HasCondition(job *batchv1.Job, jobConditionType batchv1.JobConditionType) bool {
	for _, cond := range job.Status.Conditions {
		if cond.Type == jobConditionType {
			return true
		}
	}

	return false
}

func (r *KrokEventReconciler) ConditionReason(job *batchv1.Job, jobConditionType batchv1.JobConditionType) string {
	for _, cond := range job.Status.Conditions {
		if cond.Type == jobConditionType {
			return cond.Message
		}
	}

	return ""
}

func (r *KrokEventReconciler) generateJobName(eventName, commandName string) string {
	return fmt.Sprintf("%s-job-%s", eventName, commandName)
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrokEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KrokEvent{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// reconcileCreateJobs starts the jobs in a suspended state. Only after they are reconciled can they start.
func (r *KrokEventReconciler) createJob(ctx context.Context, logger logr.Logger, command *v1alpha1.KrokCommand, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository, url string) error {
	logger.V(4).Info("launching the following command", "command", klog.KObj(command))

	args := []string{
		fmt.Sprintf("--platform=%s", repository.Spec.Platform),
		fmt.Sprintf("--event-type=%s", event.Spec.Type),
		fmt.Sprintf("--payload=%s", event.Spec.Payload),
		fmt.Sprintf("--artifact-url=%s", url),
	}
	var err error
	args, err = r.readInputFromSecrets(ctx, args, command)
	if err != nil {
		return fmt.Errorf("failed to add user defined arguments: %w", err)
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.GroupName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.generateJobName(event.Name, command.Name),
			Namespace: command.Namespace,
			Annotations: map[string]string{
				krokAnnotationKey: krokAnnotationValue,
				ownerCommandName:  command.Name,
			},
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: pointer.Int64(int64(r.CommandTimeout)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						outputSecretAnnotationKey: fmt.Sprintf("%t", command.Spec.CommandHasOutputToWrite),
						krokAnnotationKey:         krokAnnotationValue,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "command",
							Image: command.Spec.Image,
							Args:  args,
						},
					},
					RestartPolicy: "Never",
				},
			},
			TTLSecondsAfterFinished: pointer.Int32(120),
			Suspend:                 pointer.Bool(true),
		},
	}
	// Only add the finalizer in case we need the output.
	if command.Spec.CommandHasOutputToWrite {
		job.Spec.Template.Finalizers = []string{finalizer}
	}

	// Add all dependencies as a list of command names.
	if len(command.Spec.Dependencies) > 0 {
		var result []string
		for _, dependency := range command.Spec.Dependencies {
			result = append(result, r.generateJobName(event.Name, dependency))
		}
		job.ObjectMeta.Annotations[dependenciesKey] = strings.Join(result, ",")
	}
	// Set external object ControllerReference to the provider ref.
	if err := controllerutil.SetControllerReference(event, job, r.Client.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (r *KrokEventReconciler) readInputFromSecrets(ctx context.Context, args []string, command *v1alpha1.KrokCommand) ([]string, error) {
	if len(command.Spec.ReadInputFromSecrets) == 0 {
		return args, nil
	}
	for _, input := range command.Spec.ReadInputFromSecrets {
		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.Name,
		}, secret); err != nil {
			return nil, fmt.Errorf("failed to get secret which was requested: %w", err)
		}
		// process output from other commands
		if data, ok := secret.Data["output"]; ok {
			s := string(data)
			beginIndex := strings.Index(s, beginOutputFormat)
			if beginIndex == -1 {
				continue
			}

			endIndex := strings.Index(s, endOutputFormat)
			between := s[beginIndex+len(beginOutputFormat)+1 : endIndex]
			split := strings.Split(between, "\n")
			for _, part := range split {
				// I'm creating the job, so I know there is only a single container specification.
				args = append(args, fmt.Sprintf("--%s", part))
			}
		} else {
			// process manually provided key values
			for k, v := range secret.Data {
				args = append(args, fmt.Sprintf("--%s=%s", k, v))
			}
		}
	}
	return args, nil
}

// artifactURL will fetch the code content based on the given repository parameters.
func (r *KrokEventReconciler) artifactURL(event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) (string, error) {
	provider, ok := r.PlatformProviders[repository.Spec.Platform]
	if !ok {
		return "", fmt.Errorf("platform %q not supported", repository.Spec.Platform)
	}

	artifactURL, err := r.SourceController.FetchCode(provider, event, repository)
	if err != nil {
		return "", fmt.Errorf("failed to fetch code: %w", err)
	}

	return artifactURL, nil
}

func (r *KrokEventReconciler) reconcileDelete(ctx context.Context, event *v1alpha1.KrokEvent) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx).WithValues("event", klog.KObj(event))

	log.Info("deleting event and jobs")

	// Remove our finalizer from the list and update it
	controllerutil.RemoveFinalizer(event, finalizer)

	if err := r.Update(ctx, event); err != nil {
		log.Error(err, "failed to update event to remove the finalizer")
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, err
	}

	log.Info("removed finalizer from event")
	// Stop reconciliation as the object is being deleted
	return ctrl.Result{}, nil
}
