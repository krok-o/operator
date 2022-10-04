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

	"github.com/krok-o/operator/api/v1alpha1"
)

var jobFinalizer = "finalizers.krok.app"

// KrokEventReconciler reconciles a KrokEvent object
type KrokEventReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	CommandTimeout int
}

//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.krok.app,resources=krokevents/finalizers,verbs=update

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

	if event.Status.Ready {
		log.Info("skip event as it's already done")
		return ctrl.Result{}, nil
	}

	repository := &v1alpha1.KrokRepository{}
	if err := GetParentObject(ctx, r.Client, "KrokRepository", v1alpha1.GroupVersion.Group, event, repository); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find parent for event: %w", err)
	}

	log.V(4).Info("found repository", "repository", klog.KObj(repository))
	if len(event.Spec.Jobs.Items) == 0 {
		if err := r.reconcileCreateJobs(ctx, log, event, repository); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create jobs: %w", err)
		}
	} else {
		done, err := r.reconcileExistingJobs(ctx, log, event, repository)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile existing jobs: %w", err)
		}
		// If the event is not done yet, reconcile it in a minute.
		if !done {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, nil
		}
	}

	return ctrl.Result{}, nil
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

func (r *KrokEventReconciler) reconcileCreateJobs(ctx context.Context, logger logr.Logger, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) error {
	// Initialize the patch helper.
	// This has to be initialized before updating the status.
	patchHelper, err := patch.NewHelper(event, r.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}
	supportsPlatform := func(platforms []string) bool {
		for _, p := range platforms {
			if p == repository.Spec.Platform {
				return true
			}
		}

		return false
	}
	for _, command := range repository.Spec.Commands.Items {
		if !command.Spec.Enabled {
			logger.Info("skipped command as it was disabled", "command", klog.KObj(&command))
			continue
		}
		if !supportsPlatform(command.Spec.Platforms) {
			logger.Info(
				"skipped command as it does not support the given platform",
				"command",
				klog.KObj(&command),
				"platform",
				repository.Spec.Platform,
				"supported-platforms",
				command.Spec.Platforms,
			)
			continue
		}
		logger.V(4).Info("launching the following command", "command", klog.KObj(&command))

		// TODO: If the command requires access to the source it must clone it itself.
		// This is not very effective... There should be access to the source to all Jobs through
		// GitRepository or something like that. I could use Flux to watch the GitRepository for changes, but that would
		// be a problem, because we need the state of the code in the specific commit hash for which this event was
		// triggered for. Ahhh!!
		args := []string{
			fmt.Sprintf("--platform=%s", repository.Spec.Platform),
			fmt.Sprintf("--event-type=%s", event.Spec.Type),
			fmt.Sprintf("--payload=%s", event.Spec.Payload),
		}

		job := &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: batchv1.GroupName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:       r.generateJobName(command.Name),
				Namespace:  command.Namespace,
				Finalizers: []string{jobFinalizer}, // to prevent the jobs from being deleted until reconciled.
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
						//ServiceAccountName:            "",
					},
				},
				TTLSecondsAfterFinished: pointer.Int32(0),
			},
		}

		// Set external object ControllerReference to the provider ref.
		if err := controllerutil.SetControllerReference(event, job, r.Client.Scheme()); err != nil {
			return fmt.Errorf("failed to set owner reference: %w", err)
		}
		event.Spec.Jobs.Items = append(event.Spec.Jobs.Items, *job)
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
	for _, job := range event.Spec.Jobs.Items {
		// refresh the status of jobs
		job := job
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: job.Namespace,
			Name:      job.Name,
		}, event); err != nil {
			return false, fmt.Errorf("failed to get event object: %w", err)
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

	if done {
		event.Status.Ready = true
		event.Status.Outcome = "succeeded"
		if failed {
			event.Status.Outcome = "failed"
		}

		// All the jobs finished. Remove their finalizers so they can be deleted.
		for _, job := range event.Spec.Jobs.Items {
			job := job
			if err := r.Client.Get(ctx, types.NamespacedName{
				Namespace: job.Namespace,
				Name:      job.Name,
			}, event); err != nil {
				return false, fmt.Errorf("failed to get event object: %w", err)
			}
			job.Finalizers = nil
			if err := r.Update(ctx, &job); err != nil {
				return false, fmt.Errorf("failed to remove finalizer from job: %w", err)
			}
		}

		logger.V(4).Info("removed all finalizers from jobs")
	}

	// Patch the source object.
	if err := patchHelper.Patch(ctx, event); err != nil {
		return done, fmt.Errorf("failed to patch repository object: %w", err)
	}
	return done, nil
}
