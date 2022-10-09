package controllers

import (
	"context"
	"fmt"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
	if !r.OwnedByKrok(job) {
		// We won't reconcile every job, just jobs which were launched by Krok.
		return ctrl.Result{}, nil
	}
	log.V(4).Info("found job object", "job", klog.KObj(job))

	// if we are still running, leave it and check back later.
	if job.Status.Active > 0 {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	// if we are no longer running, update the owner event with output from the job.
	owner := &v1alpha1.KrokEvent{}
	if err := GetParentObject(ctx, r.Client, "KrokEvent", v1alpha1.GroupVersion.Group, job, owner); err != nil {
		return ctrl.Result{}, fmt.Errorf("job has no owner: %w", err)
	}
	log.V(4).Info("found owner", "owner", klog.KObj(owner))

	config, err := rest.InClusterConfig()
	if err != nil {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("failed to get in cluster config: %w", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("failed to create clientset: %w", err)
	}
	// job-name=slack-command-job-1665307554
	pods, err := clientset.CoreV1().Pods(job.Namespace).List(ctx,
		v1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", job.Name)})

	getLogs := func(pod *corev1.Pod) ([]byte, error) {
		podReq := clientset.CoreV1().Pods(job.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
		podLogs, err := podReq.Stream(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get logs for job pod '%s': %w", pod.Name, err)
		}
		defer podLogs.Close()
		content, err := io.ReadAll(podLogs)
		if err != nil {
			return nil, fmt.Errorf("failed to read logs '%s': %w", pod.Name, err)
		}
		return content, nil
	}
	for _, pod := range pods.Items {
		podLogs, err := getLogs(&pod)
		if err != nil {
			return ctrl.Result{}, err
		}
		// update the owner and add the output.
	}

	// At the end, patch the owner.

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
