package controllers

import (
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodReconciler reconciles Pod objects
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("pod-controller")
	log.V(4).Info("running reconcile loop for pod")

	pod := &corev1.Pod{}
	if err := r.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get pod: %w", err)
	}

	if !OwnedByKrok(pod.Annotations) {
		return ctrl.Result{}, nil
	}

	log = log.WithValues("pod", klog.KObj(pod))
	log.V(4).Info("found pod")

	if v, ok := pod.Annotations[outputSecretAnnotationKey]; !ok || v != "true" {
		return ctrl.Result{}, nil
	}

	// reschedule as the pod is still working
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
		return ctrl.Result{
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	log.V(4).Info("pod is done, starting to gather logs from it")
	if err := r.outputIntoSecret(ctx, pod); err != nil {
		return ctrl.Result{
			RequeueAfter: 30 * time.Second,
		}, fmt.Errorf("failed to generate secret output: %w", err)
	}

	controllerutil.RemoveFinalizer(pod, finalizer)
	if err := r.Client.Update(ctx, pod); err != nil {
		log.Error(err, "failed remove finalizer from pod")
		return ctrl.Result{
			RequeueAfter: 20 * time.Second,
		}, fmt.Errorf("failed to update job: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Or(PodUpdatePredicate{})).
		Complete(r)
}

func (r *PodReconciler) outputIntoSecret(ctx context.Context, pod *corev1.Pod) error {
	secretExists := true
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      r.generateJobSecretName(pod.Labels),
		Namespace: pod.Namespace,
	}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			secretExists = false
			secret = &corev1.Secret{
				TypeMeta: v1.TypeMeta{
					Kind:       "Secret",
					APIVersion: corev1.GroupName,
				},
				ObjectMeta: v1.ObjectMeta{
					Name:      r.generateJobSecretName(pod.Labels),
					Namespace: pod.Namespace,
				},
				Type: "opaque",
			}
		}
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

	getLogs := func(pod *corev1.Pod) ([]byte, error) {
		podReq := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
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
	podLogs, err := getLogs(pod)
	if err != nil {
		return fmt.Errorf("failed to get pod logs: %w", err)
	}
	// TODO: This will override whatever data is in there. Which means if it exists, I need the data and append to it.
	// But technically, it's only a single pod so should be okay.
	secret.Data = map[string][]byte{
		"output": podLogs,
	}
	if secretExists {
		if err := r.Update(ctx, secret); err != nil {
			return fmt.Errorf("failed to update secret with new output: %w", err)
		}
	} else {
		if err := r.Create(ctx, secret); err != nil {
			return fmt.Errorf("failed to create output secret: %w", err)
		}
	}
	return nil
}

func (r *PodReconciler) generateJobSecretName(labels map[string]string) string {
	return labels["job-name"]
}
