package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodDeletePredicate triggers an update event when a Pod is deleted.
type PodDeletePredicate struct {
	predicate.Funcs
}

func (PodDeletePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	src, ok := e.ObjectNew.(*corev1.Pod)
	if !ok {
		return false
	}

	return e.ObjectOld.GetGeneration() < e.ObjectNew.GetGeneration() && src.DeletionTimestamp != nil
}
