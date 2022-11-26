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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitLab contains GitLab specific settings.
type GitLab struct {
	// ProjectID is an optional ID which defines a project in Gitlab.
	ProjectID int `json:"projectID,omitempty"`
}

// Ref points to a secret which contains access information for the repository.
type Ref struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type CommandRef struct {
	Ref `json:",inline"`
}

// KrokRepositorySpec defines the desired state of KrokRepository
type KrokRepositorySpec struct {
	// URL of the repository.
	// +kubebuilder:validation:Required
	URL string `json:"url"`
	// Platform defines on which platform this repository is in. Exp.: GitHub, GitLab, Gitea...
	// +kubebuilder:validation:Required
	Platform string `json:"platform"`
	// GitLab specific settings.
	// +optional
	GitLab *GitLab `json:"gitLab,omitempty"`
	// AuthSecretRef contains the ref to the secret containing authentication data for this repository.
	// +kubebuilder:validation:Required
	AuthSecretRef Ref `json:"authSecretRef"`
	// ProviderTokenSecretRef contains the ref to the secret containing authentication data for the provider of this
	// repository. For example, GitHub token, or a Gitlab token...
	// +kubebuilder:validation:Required
	ProviderTokenSecretRef Ref `json:"providerTokenSecretRef"`
	// Commands contains all the commands which this repository is attached to.
	// +optional
	Commands []CommandRef `json:"commands,omitempty"`
	// Events contains all events that this repository subscribes to.
	Events []string `json:"events,omitempty"`
	// EventReconcileInterval can be set to define how often a created KrokEvent should requeue itself.
	EventReconcileInterval string `json:"eventReconcileInterval,omitempty"`
}

// KrokRepositoryStatus defines the observed state of KrokRepository
type KrokRepositoryStatus struct {
	// A Unique URL for this given repository. Generated upon creation and saved in Status field.
	UniqueURL string `json:"uniqueURL,omitempty"`
	// Events contains run outputs for command runs that have been executed for this Repository.
	// This holds the last 10 outcomes.
	Events KrokEventList `json:"events,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KrokRepository is the Schema for the krokrepositories API
type KrokRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KrokRepositorySpec   `json:"spec,omitempty"`
	Status KrokRepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KrokRepositoryList contains a list of KrokRepository
type KrokRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KrokRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KrokRepository{}, &KrokRepositoryList{})
}
