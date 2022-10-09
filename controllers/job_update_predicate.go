package controllers

import (
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// JobUpdatePredicate triggers an update event when status of a job changes.
type JobUpdatePredicate struct {
	predicate.Funcs
}

func (JobUpdatePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	oldJob, ok := e.ObjectOld.(*batchv1.Job)
	if !ok {
		return false
	}

	newJob, ok := e.ObjectNew.(*batchv1.Job)
	if !ok {
		return false
	}

	if oldJob.Status.Conditions == nil && newJob.Status.Conditions != nil {
		return true
	}

	return false
}
