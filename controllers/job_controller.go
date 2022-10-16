package controllers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/krok-o/operator/api/v1alpha1"
)

var (
	krokAnnotationValue = "krokjob"
	krokAnnotationKey   = "krok.app"
	ownerCommandName    = "command.name"
	dependenciesKey     = "jobDependencies"
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
	if !r.OwnedByKrok(job) {
		// We won't reconcile every job, just jobs which were launched by Krok.
		return ctrl.Result{}, nil
	}
	log.V(4).Info("found job object", "job", klog.KObj(job))

	resumeJob := func(ctx context.Context, job *batchv1.Job) (ctrl.Result, error) {
		// Need to re-get the job, because it was potentially updated
		// when it was looking for an output and adds the arguments.
		if err := r.Get(ctx, types.NamespacedName{
			Name:      job.Name,
			Namespace: job.Namespace,
		}, job); err != nil {
			log.Info("failed to get depending job, marking this job as failed")
			return ctrl.Result{}, nil
		}
		job.Spec.Suspend = pointer.Bool(false)
		if err := r.Update(ctx, job); err != nil {
			return ctrl.Result{
				RequeueAfter: 10 * time.Second,
			}, fmt.Errorf("failed to unsuspend job: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// If suspended...
	if job.Spec.Suspend == nil || *job.Spec.Suspend {
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
				log.Info("failed to find depending job, marking this job as failed")
				// TODO: Figure out how to fail a job.
				return ctrl.Result{}, nil
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

		// TODO: Fail the Job once you figure out how to.
		return ctrl.Result{
			RequeueAfter: 5 * time.Second,
		}, nil
	}

	// if we are still running, leave it and check back later.
	if job.Status.Active > 0 {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, nil
	}

	// we are no longer running
	// - if there is a secret defined for output, put the output in there
	// - update the parent event and set its status to DONE
	owner := &v1alpha1.KrokEvent{}
	if err := GetParentObject(ctx, r.Client, "KrokEvent", v1alpha1.GroupVersion.Group, job, owner); err != nil {
		return ctrl.Result{}, fmt.Errorf("job has no owner: %w", err)
	}
	log.V(4).Info("found owner", "owner", klog.KObj(owner))

	commandName, ok := job.Annotations[ownerCommandName]
	if !ok {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("job doesn't have an owning command")
	}
	command := &v1alpha1.KrokCommand{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      commandName,
		Namespace: job.Namespace,
	}, command); err != nil {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("failed to find command: %w", err)
	}

	if command.Spec.CommandHasOutputToWrite {
		if err := r.outputIntoSecret(ctx, job); err != nil {
			return ctrl.Result{
				RequeueAfter: 30 * time.Second,
			}, fmt.Errorf("failed to generate secret output: %w", err)
		}
	}

	patchHelper, err := patch.NewHelper(owner, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	// update the owner and add the output
	owner.Status.Done = true

	// Patch the owner object.
	if err := patchHelper.Patch(ctx, owner); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch event object: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *JobReconciler) outputIntoSecret(ctx context.Context, job *batchv1.Job) error {
	secret := &corev1.Secret{
		TypeMeta: v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.GroupName,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      r.generateJobSecretName(job),
			Namespace: job.Namespace,
		},
		Type: "opaque",
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in cluster config: %w", err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}
	// job-name=slack-command-job-1665307554
	pods, err := clientset.CoreV1().Pods(job.Namespace).List(ctx,
		v1.ListOptions{LabelSelector: fmt.Sprintf("job-name=%s", job.Name)})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

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
	var compoundedLogs string
	for _, pod := range pods.Items {
		podLogs, err := getLogs(&pod)
		if err != nil {
			return fmt.Errorf("failed to get pod logs: %w", err)
		}
		compoundedLogs += string(podLogs) + "/n"
	}
	secret.Data = map[string][]byte{
		"output": []byte(compoundedLogs),
	}
	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create output secret: %w", err)
	}
	return nil
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
	endIndex := strings.Index(s, endOutputFormat)
	between := s[beginIndex+len(beginOutputFormat)+1 : endIndex]
	split := strings.Split(between, "\n")
	for _, part := range split {
		// I'm creating the job, so I know there is only a single container specification.
		if len(job.Spec.Template.Spec.Containers) == 0 {
			return fmt.Errorf("container specification for job is empty")
		}
		container := job.Spec.Template.Spec.Containers[0]
		container.Args = append(container.Args, fmt.Sprintf("--%s", part))
	}
	if err := r.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job to add output: %w", err)
	}
	return nil
}
