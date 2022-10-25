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
)

var (
	krokAnnotationValue = "krok-job"
	krokAnnotationKey   = "krok-app"
	ownerCommandName    = "command-name"
	dependenciesKey     = "job-dependencies"
	//outputKey           = "jobOutput"
	beginOutputFormat = "----- BEGIN OUTPUT -----"
	endOutputFormat   = "----- END OUTPUT -----"
)

// JobReconciler reconciles Job objects
type JobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *JobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("job-controller")
	log.V(4).Info("running reconcile loop for jobs")
	job := &batchv1.Job{}
	if err := r.Client.Get(ctx, req.NamespacedName, job); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get job object: %w", err)
	}
	if !OwnedByKrok(job.Annotations) {
		// We won't reconcile every job, just jobs which were launched by Krok.
		return ctrl.Result{}, nil
	}
	log = log.WithValues("job", klog.KObj(job))
	log.V(4).Info("found job object")
	if job.DeletionTimestamp != nil {
		log.Info("job is being deleted, remove the finalizer.")
		newJob := job.DeepCopy()
		controllerutil.RemoveFinalizer(newJob, finalizer)
		if err := r.patchObject(ctx, job, newJob); err != nil {
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, fmt.Errorf("failed to patch object: %w", err)
		}
		return ctrl.Result{}, nil
	}

	owner := &v1alpha1.KrokEvent{}
	if err := GetParentObject(ctx, r.Client, "KrokEvent", v1alpha1.GroupVersion.Group, job, owner); err != nil {
		return ctrl.Result{}, fmt.Errorf("job has no owner: %w", err)
	}
	log.V(4).Info("found owner", "owner", klog.KObj(owner))

	resumeJob := func(ctx context.Context, job *batchv1.Job) (ctrl.Result, error) {
		newJob := job.DeepCopy()
		newJob.Spec.Suspend = pointer.Bool(false)
		if err := r.patchObject(ctx, job, newJob); err != nil {
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, fmt.Errorf("failed to patch object: %w", err)
		}
		return ctrl.Result{}, nil
	}

	failOwnerEvent := func(ctx context.Context, event *v1alpha1.KrokEvent) (ctrl.Result, error) {
		newEvent := event.DeepCopy()
		event.Status.Done = true
		event.Status.Outcome = "Failed"
		if err := r.patchObject(ctx, event, newEvent); err != nil {
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, fmt.Errorf("failed to patch object: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// If suspended...
	if job.Spec.Suspend != nil && *job.Spec.Suspend {
		dependingJobNames, ok := job.Annotations[dependenciesKey]
		if !ok {
			return resumeJob(ctx, job)
		}
		split := strings.Split(dependingJobNames, ",")
		resume := true
		for _, dependingJobName := range split {
			dependingJob := &batchv1.Job{}
			if err := r.Get(ctx, types.NamespacedName{
				Name:      dependingJobName,
				Namespace: job.Namespace,
			}, dependingJob); err != nil {
				if apierrors.IsNotFound(err) {
					// Give it a bit of time to find the dependent job.
					return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
				}
				log.Error(err, "failed to find depending job, marking this job as failed")
				// TODO: Figure out how to fail a job.
				// Alternatively, just ignore the job for now. It will be deleted later.
				// And possibly, mark the owner event failed?
				return failOwnerEvent(ctx, owner)
			}
			if dependingJob.Status.CompletionTime == nil {
				resume = false
				continue
			}
			if err := r.updateJobWithOutput(ctx, log, job); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to get job output: %w", err)
			}
		}
		if resume {
			return resumeJob(ctx, job)
		}

		// re-queue as the depending on jobs aren't done yet.
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	// if we are still running, leave it and check back later.
	if job.Status.Active > 0 {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	// ZagZag
	jobsDone := true
	for _, job := range owner.Status.Jobs {
		j := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      job.Name,
			Namespace: job.Namespace,
		}, j); err != nil {
			if apierrors.IsNotFound(err) {
				log.V(4).Info("job not found", "job", job)
				continue
			}
			log.Info("re-queuing event as its jobs aren't done yet")
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, err
		}
		// We just check if all jobs succeeded here or not. If not, the event will be done later and status will be
		// updated to Failed.
		if j.Status.CompletionTime == nil {
			jobsDone = false
			break
		}
	}

	if !jobsDone {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}
	newOwner := owner.DeepCopy()
	newOwner.Status.Done = true
	newOwner.Status.Outcome = "Success"

	// Patch the owner object.
	if err := r.patchObject(ctx, owner, newOwner); err != nil {
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, fmt.Errorf("failed to patch object: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.Job{}).
		WithEventFilter(predicate.Or(JobUpdatePredicate{}, JobDeletePredicate{})).
		Complete(r)
}

func (r *JobReconciler) generateJobSecretName(job *batchv1.Job) string {
	return fmt.Sprintf("%s-secret", job.Name)
}

func (r *JobReconciler) updateJobWithOutput(ctx context.Context, log logr.Logger, job *batchv1.Job) error {
	secretName := r.generateJobSecretName(job)
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: job.Namespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// If the secret is found, get the output out of it and provide it.
	data, ok := secret.Data["output"]
	if !ok {
		return fmt.Errorf("secret didn't contain 'output'")
	}
	s := string(data)
	beginIndex := strings.Index(s, beginOutputFormat)
	if beginIndex == -1 {
		log.Info("secret didn't contain any values to process", "secret", klog.KObj(secret))
		return nil
	}

	patchHelper, err := patch.NewHelper(job, r.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}

	endIndex := strings.Index(s, endOutputFormat)
	between := s[beginIndex+len(beginOutputFormat)+1 : endIndex]
	split := strings.Split(between, "\n")
	for _, part := range split {
		// I'm creating the job, so I know there is only a single container specification.
		if len(job.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("container specification for job is empty")
		}
		job.Spec.Template.Spec.Containers[0].Args = append(job.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("--%s", part))
	}

	if err := patchHelper.Patch(ctx, job); err != nil {
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
}

func (r *JobReconciler) patchObject(ctx context.Context, oldObject, newObject client.Object) error {
	patchHelper, err := patch.NewHelper(oldObject, r.Client)
	if err != nil {
		return fmt.Errorf("failed to create patch helper: %w", err)
	}
	if err := patchHelper.Patch(ctx, newObject); err != nil {
		return fmt.Errorf("failed to patch object: %w", err)
	}
	return nil
}
