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
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
	source_controller "github.com/krok-o/operator/pkg/source-controller"
)

var jobFinalizer = "event.krok.app/finalizer"

// KrokEventReconciler reconciles a KrokEvent object
type KrokEventReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CommandTimeout    int
	PlatformProviders map[string]providers.Platform
	SourceController  *source_controller.Server
}

//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/finalizers,verbs=update
//+kubebuilder:rbac:groups=delivery.krok.app,resources=jobs/finalizers,verbs=update

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

	if event.Status.Done {
		log.Info("skip event as it's already done")
		return ctrl.Result{}, nil
	}

	repository := &v1alpha1.KrokRepository{}
	if err := GetParentObject(ctx, r.Client, "KrokRepository", v1alpha1.GroupVersion.Group, event, repository); err != nil {
		return ctrl.Result{
			RequeueAfter: 20 * time.Second,
		}, fmt.Errorf("failed to find parent for event: %w", err)
	}

	log.V(4).Info("found repository", "repository", klog.KObj(repository))
	if len(event.Status.Jobs) == 0 {
		log.V(4).Info("event has no jobs, creating them", "event", klog.KObj(event))
		artifactURL, err := r.reconcileSource(event, repository)
		if err != nil {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("failed to checkout source: %w", err)
		}
		if err := r.reconcileCreateJobs(ctx, log, event, repository, artifactURL); err != nil {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("failed to create jobs: %w", err)
		}
	} else {
		done, err := r.reconcileExistingJobs(ctx, log, event, repository)
		if err != nil {
			return ctrl.Result{
				RequeueAfter: 20 * time.Second,
			}, fmt.Errorf("failed to reconcile existing jobs: %w", err)
		}
		// If the event is not done yet, reconcile it in a minute.
		if !done {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, nil
		}
	}

	// TODO: This is why it never reconciles this event again. So we basically need to constantly check events
	// because we use them. Or, we create some kind of dynamic watch for each job. Which ever will be more effective.
	return ctrl.Result{
		RequeueAfter: 30 * time.Second,
	}, nil
}

func (r *KrokEventReconciler) generateJobName(commandName string) string {
	return fmt.Sprintf("%s-job-%d", commandName, time.Now().Unix())
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrokEventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KrokEvent{}).
		WithEventFilter(predicate.Or(ArtifactUpdatePredicate{}, DeletePredicate{})).
		Complete(r)
}

func (r *KrokEventReconciler) reconcileCreateJobs(ctx context.Context, logger logr.Logger, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository, url string) error {
	supportsPlatform := func(platforms []string) bool {
		for _, p := range platforms {
			if p == repository.Spec.Platform {
				return true
			}
		}

		return false
	}
	var jobList []v1alpha1.Ref
	for _, commandRef := range repository.Spec.Commands {
		command := &v1alpha1.KrokCommand{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: commandRef.Namespace,
			Name:      commandRef.Name,
		}, command); err != nil {
			return fmt.Errorf("failed to get command object: %w", err)
		}
		if !command.Spec.Enabled {
			logger.Info("skipped command as it was disabled", "command", klog.KObj(command))
			continue
		}
		if !supportsPlatform(command.Spec.Platforms) {
			logger.Info(
				"skipped command as it does not support the given platform",
				"command",
				klog.KObj(command),
				"platform",
				repository.Spec.Platform,
				"supported-platforms",
				command.Spec.Platforms,
			)
			continue
		}
		logger.V(4).Info("launching the following command", "command", klog.KObj(command))

		args := []string{
			fmt.Sprintf("--platform=%s", repository.Spec.Platform),
			fmt.Sprintf("--event-type=%s", event.Spec.Type),
			fmt.Sprintf("--payload=%s", event.Spec.Payload),
			fmt.Sprintf("--artifact-url=%s", url),
		}

		job := &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: batchv1.GroupName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.generateJobName(command.Name),
				Namespace: command.Namespace,
			},
			Spec: batchv1.JobSpec{
				ActiveDeadlineSeconds: pointer.Int64(int64(r.CommandTimeout)),
				Template: corev1.PodTemplateSpec{
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
				TTLSecondsAfterFinished: pointer.Int32(0),
			},
		}

		// Set external object ControllerReference to the provider ref.
		if err := controllerutil.SetControllerReference(event, job, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}

		// Since this is a brand-new object, we can be sure that it will be an update.
		controllerutil.AddFinalizer(job, jobFinalizer)

		if err := r.Create(ctx, job); err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}
		jobList = append(jobList, v1alpha1.Ref{
			Name:      job.Name,
			Namespace: job.Namespace,
		})
	}

	// Initialize the patch helper.
	// This has to be initialized before updating the status.
	patchHelper, err := patch.NewHelper(event, r.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}

	event.Status = v1alpha1.KrokEventStatus{
		Jobs: jobList,
		Done: false,
	}

	// Patch the source object.
	if err := patchHelper.Patch(ctx, event); err != nil {
		return fmt.Errorf("failed to patch event object: %w", err)
	}

	return nil
}

func (r *KrokEventReconciler) reconcileExistingJobs(ctx context.Context, logger logr.Logger, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) (bool, error) {
	// Initialize the patch helper.
	// This has to be initialized before updating the status.
	patchHelper, err := patch.NewHelper(event, r.Client)
	if err != nil {
		return false, fmt.Errorf("failed to create patch helper: %w", err)
	}

	var (
		done   = true
		failed bool
	)
	for i := 0; i < len(event.Status.Jobs); i++ {
		// refresh the status of jobs
		jobRef := event.Status.Jobs[i]

		job := &batchv1.Job{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: jobRef.Namespace,
			Name:      jobRef.Name,
		}, job); err != nil {
			if apierrors.IsNotFound(err) {
				// Job has been removed manually, remove it from the list.
				event.Status.Jobs = append(event.Status.Jobs[:i], event.Status.Jobs[i+1:]...)
				i--
				continue
			}
			return false, fmt.Errorf("failed to get job object: %w", err)
		}
		if job.Status.Active > 0 {
			done = false
		} else {
			// look through the conditions of this job. If it has any "failed" conditions,
			// we set the end result to failed.
			if job.Status.Failed > 0 {
				failed = true
			}
		}
	}

	event.Status.Done = false
	if done {
		event.Status.Done = true
		event.Status.Outcome = "succeeded"
		if failed {
			event.Status.Outcome = "failed"
		}

		// All the jobs finished. Remove their finalizers so they can be deleted.
		for _, jobRef := range event.Status.Jobs {
			jobRef := jobRef

			job := &batchv1.Job{}
			if err := r.Client.Get(ctx, types.NamespacedName{
				Namespace: jobRef.Namespace,
				Name:      jobRef.Name,
			}, job); err != nil {
				return false, fmt.Errorf("failed to get job object: %w", err)
			}
			controllerutil.RemoveFinalizer(job, jobFinalizer)
			if err := r.Update(ctx, job); err != nil {
				return false, fmt.Errorf("failed to remove finalizer from job: %w", err)
			}
		}

		logger.V(4).Info("removed all finalizers from jobs")
	}

	// Patch the source object.
	if err := patchHelper.Patch(ctx, event); err != nil {
		return done, fmt.Errorf("failed to patch event object: %w", err)
	}
	return done, nil
}

var eventsWhichNeedSourceCode = []string{"push", "pull_request"}
var contains = func(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}

// reconcileSource will fetch the code content based on the given repository parameters.
func (r *KrokEventReconciler) reconcileSource(event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) (string, error) {
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

	for i := 0; i < len(event.Status.Jobs); i++ {
		job := &batchv1.Job{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: event.Status.Jobs[i].Namespace,
			Name:      event.Status.Jobs[i].Name,
		}, job); err != nil {
			if apierrors.IsNotFound(err) {
				event.Status.Jobs = append(event.Status.Jobs[:i], event.Status.Jobs[i+1:]...)
				i--
				continue
			}

			log.Error(err, "failed to remove job from event")
			return ctrl.Result{
				RequeueAfter: 20 * time.Second,
			}, fmt.Errorf("failed to remove job from event: %w", err)
		}

		// Remove our finalizer from the list and update it
		controllerutil.RemoveFinalizer(job, jobFinalizer)
		if err := r.Client.Update(ctx, job); err != nil {
			log.Error(err, "failed to remove job")
			return ctrl.Result{
				RequeueAfter: 20 * time.Second,
			}, fmt.Errorf("failed to update job: %w", err)
		}
		background := metav1.DeletePropagationBackground
		if err := r.Client.Delete(ctx, job, &client.DeleteOptions{
			PropagationPolicy: &background,
		}); err != nil {
			log.Error(err, "failed to remove job")
			return ctrl.Result{
				RequeueAfter: 20 * time.Second,
			}, fmt.Errorf("failed to remove job: %w", err)
		}
	}

	// Remove our finalizer from the list and update it
	// propagationPolicy=
	controllerutil.RemoveFinalizer(event, jobFinalizer)

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
