package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodUpdatePredicate triggers an update event when status of a pod changes.
type PodUpdatePredicate struct {
	predicate.Funcs
}

func (PodUpdatePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldPod, ok := e.ObjectOld.(*corev1.Pod)
	if !ok {
		return false
	}

	newPod, ok := e.ObjectNew.(*corev1.Pod)
	if !ok {
		return false
	}

	if oldPod.Status.Phase != newPod.Status.Phase {
		return true
	}

	if oldPod.Status.Conditions == nil && newPod.Status.Conditions != nil {
		return true
	}

	return false
}
