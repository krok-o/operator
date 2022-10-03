/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/krok-o/operator/api/v1alpha1"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	HookBase     string
	HookProtocol string
}

//+kubebuilder:rbac:groups=delivery.krok.app,resources=repositories,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=delivery.krok.app,resources=repositories/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.krok.app,resources=repositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("krok-operator")
	log.V(4).Info("starting reconcile loop")
	repository := &v1alpha1.Repository{}
	if err := r.Client.Get(ctx, req.NamespacedName, repository); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, fmt.Errorf("failed to get source object: %w", err)
	}
	log = log.WithValues("repository", repository)

	log.V(4).Info("found repository")
	if repository.Status.UniqueURL != "" {
		log.Info("skipping repository as it was already reconciled")
		return ctrl.Result{}, nil
	}

	// Generate unique callback URL.
	u, err := r.generateUniqueCallBackURL(repository)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("unique URL generation failed: %w", err)
	}

	repository.Status.UniqueURL = u

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(repository, r.Client)
	if err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to create patch helper: %w", err)
	}

	// Patch the source object.
	if err := patchHelper.Patch(ctx, repository); err != nil {
		return ctrl.Result{
			RequeueAfter: 1 * time.Minute,
		}, fmt.Errorf("failed to patch repository object: %w", err)
	}
	log.V(4).Info("patch successful")
	// Create Hook
	return ctrl.Result{}, nil
}

// generateUniqueCallBackURL takes a repository and generates a unique URL based on the ID and Type of the repo
// and the configured Krok hostname.
func (r *RepositoryReconciler) generateUniqueCallBackURL(repo *v1alpha1.Repository) (string, error) {
	u, err := url.Parse(fmt.Sprintf("%s://%s", r.HookProtocol, r.HookBase))
	if err != nil {
		return "", fmt.Errorf("failed to generate unique URL for repository: %w", err)
	}
	u.Path = path.Join(u.Path, "hooks", repo.Name, repo.Spec.Platform, "callback")
	return u.String(), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Repository{}).
		Complete(r)
}
