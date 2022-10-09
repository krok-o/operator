package controllers

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/krok-o/operator/api/v1alpha1"
)

var (
	krokAnnotationValue = "krokjob"
	krokAnnotationKey   = "krok.app"
)

// JobReconciler reconciles Job objects
type JobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx).WithName("job-controller")
	log.V(4).Info("running reconcile loop for jobs")
	job := &batchv1.Job{}
	if err := r.Client.Get(ctx, req.NamespacedName, job); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get job object: %w", err)
	}
	if !r.OwnedByKrok(job) {
		// We won't reconcile every job, just jobs which were launched by Krok.
		return ctrl.Result{}, nil
	}
	log.V(4).Info("found job object", "job", klog.KObj(job))
	owner := &v1alpha1.KrokEvent{}
	if err := GetParentObject(ctx, r.Client, "KrokEvent", v1alpha1.GroupVersion.Group, job, owner); err != nil {
		return ctrl.Result{}, fmt.Errorf("job has no owner: %w", err)
	}
	log.V(4).Info("found owner", "owner", klog.KObj(owner))
	return ctrl.Result{}, nil
}

func (r *JobReconciler) OwnedByKrok(job *batchv1.Job) bool {
	_, ok := job.Annotations[krokAnnotationKey]
	return ok
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(predicate.Or(JobUpdatePredicate{}, JobDeletePredicate{})).
		Complete(r)
}
