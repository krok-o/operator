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

	apimeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/krok-o/operator/pkg/providers"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/krok-o/operator/api/v1alpha1"
)

var jobFinalizer = "event.krok.app/finalizer"

// KrokEventReconciler reconciles a KrokEvent object
type KrokEventReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CommandTimeout    int
	PlatformProviders map[string]providers.Platform
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
		artifactURL, err := r.reconcileGitRepository(ctx, log, event, repository)
		if err != nil {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("failed to create jobs: %w", err)
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
								Name:           "command",
								Image:          command.Spec.Image,
								Args:           args,
								LivenessProbe:  nil,
								ReadinessProbe: nil,
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

		if err := r.Create(context.Background(), job); err != nil {
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
	var (
		done   = true
		failed bool
	)
	for _, jobRef := range event.Status.Jobs {
		// refresh the status of jobs
		jobRef := jobRef

		job := &batchv1.Job{}
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: jobRef.Namespace,
			Name:      jobRef.Name,
		}, job); err != nil {
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

	// Initialize the patch helper.
	// This has to be initialized before updating the status.
	patchHelper, err := patch.NewHelper(event, r.Client)
	if err != nil {
		return done, fmt.Errorf("failed to create patch helper: %w", err)
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

// reconcileGitRepository will create a GitRepository object and wait for it to be reconciled.
// Once the reconciliation of the repository is done it will return the artifact URL for the other commands to access.
func (r *KrokEventReconciler) reconcileGitRepository(ctx context.Context, logger logr.Logger, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) (string, error) {
	if !contains(eventsWhichNeedSourceCode, event.Spec.Type) {
		// Repository does not require code.
		return "", nil
	}
	gitRepository := &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitRepository",
			APIVersion: sourcev1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%d", repository.Name, event.Name, time.Now().Unix()),
			Namespace: event.Namespace,
		},
		Spec: sourcev1.GitRepositorySpec{
			URL: repository.Spec.URL,
			SecretRef: &apimeta.LocalObjectReference{
				Name: repository.Spec.AuthSecretRef.Name,
			},
			Interval: metav1.Duration{
				Duration: 30 * time.Minute,
			},
		},
	}

	provider, ok := r.PlatformProviders[repository.Spec.Platform]
	if !ok {
		return "", fmt.Errorf("platform %q not supported", repository.Spec.Platform)
	}

	ref, err := provider.GetRefIfPresent(ctx, event)
	if err != nil {
		return "", fmt.Errorf("failed to get ref for event: %w", err)
	}

	if ref == "not-found" {
		// There is no ref.
		return "", nil
	}

	if strings.Contains(ref, "tags") {
		split := strings.Split(ref, "/")
		gitRepository.Spec.Reference = &sourcev1.GitRepositoryRef{
			Tag: split[len(split)-1],
		}
	} else if strings.Contains(ref, "heads") {
		split := strings.Split(ref, "/")
		gitRepository.Spec.Reference = &sourcev1.GitRepositoryRef{
			Branch: split[len(split)-1],
		}
	}

	if err := r.Create(ctx, gitRepository); err != nil {
		return "", fmt.Errorf("failed to create GitRepository: %w", err)
	}
	logger.V(4).Info("waiting for gitRepository to be reconciled")
	if err := wait.PollImmediate(2*time.Second, 1*time.Minute,
		func() (done bool, err error) {
			namespacedName := types.NamespacedName{
				Namespace: gitRepository.GetNamespace(),
				Name:      gitRepository.GetName(),
			}
			if err := r.Get(ctx, namespacedName, gitRepository); err != nil {
				return false, err
			}
			return meta.IsStatusConditionTrue(gitRepository.Status.Conditions, apimeta.ReadyCondition), nil
		}); err != nil {
		return "", fmt.Errorf("failed to reconcile GitRepository: %w", err)
	}
	if gitRepository.Status.Artifact == nil {
		return "", fmt.Errorf("failed to reconcile artifact, was nil")
	}
	return gitRepository.Status.Artifact.URL, nil
}
