package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/krok-o/operator/api/v1alpha1"
)

// DeletePredicate triggers an update event when a HelmRepository generation changes.
// i.e.: Delete events.
type DeletePredicate struct {
	predicate.Funcs
}

func (DeletePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	src, ok := e.ObjectNew.(*v1alpha1.KrokEvent)
	if !ok {
		return false
	}

	return e.ObjectOld.GetGeneration() < e.ObjectNew.GetGeneration() && src.DeletionTimestamp != nil
}
