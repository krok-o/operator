package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/krok-o/operator/api/v1alpha1"
)

// ArtifactUpdatePredicate triggers an update event when a KrokEvent jobs revision changes.
type ArtifactUpdatePredicate struct {
	predicate.Funcs
}

func (ArtifactUpdatePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldEvent, ok := e.ObjectOld.(*v1alpha1.KrokEvent)
	if !ok {
		return false
	}

	newEvent, ok := e.ObjectNew.(*v1alpha1.KrokEvent)
	if !ok {
		return false
	}

	if oldEvent.Status.Jobs == nil && newEvent.Status.Jobs != nil {
		return true
	}

	return false
}
