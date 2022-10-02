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

// RepositorySpec defines the desired state of Repository
type RepositorySpec struct {
	// Name of the repository.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// URL of the repository.
	// +kubebuilder:validation:Required
	URL string `json:"url"`
	// VCS Defines which handler will be used. For values, see platforms.go.
	// +kubebuilder:validation:Required
	VCS int `json:"vcs"`
	// GitLab specific settings.
	// +optional
	GitLab *GitLab `json:"gitLab,omitempty"`
	// AuthSecretRef contains the ref to the secret containing authentication data for this repository.
	// +kubebuilder:validation:Required
	AuthSecretRef string `json:"auth,omitempty"`
	// Commands contains all the commands which this repository is attached to.
	// +optional
	Commands *CommandList `json:"commands,omitempty"`
}

// RepositoryStatus defines the observed state of Repository
type RepositoryStatus struct {
	// required: true
	UniqueURL string `json:"unique_url,omitempty"`
	// Events contains all events that are being executed or were executed for this repository.
	Events []string `json:"events,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Repository is the Schema for the repositories API
type Repository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepositorySpec   `json:"spec,omitempty"`
	Status RepositoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RepositoryList contains a list of Repository
type RepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Repository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Repository{}, &RepositoryList{})
}
