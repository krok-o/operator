package controllers

import (
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// JobDeletePredicate triggers an update event when a Job is deleted.
type JobDeletePredicate struct {
	predicate.Funcs
}

func (JobDeletePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	src, ok := e.ObjectNew.(*batchv1.Job)
	if !ok {
		return false
	}

	return e.ObjectOld.GetGeneration() < e.ObjectNew.GetGeneration() && src.DeletionTimestamp != nil
}
