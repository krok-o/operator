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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CommandSpec defines the desired state of Command
type CommandSpec struct {
	// Name of the command.
	Name string `json:"name"`
	// Schedule of the command.
	// example: 0 * * * * // follows cron job syntax.
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// Repositories that this command can execute on.
	// +optional
	Repositories RepositoryList `json:"repositories,omitempty"`
	// Image defines the image name and tag of the command
	// example: krok-hook/slack-notification:v0.0.1
	Image string `json:"image"`
	// Enabled defines if this command can be executed or not.
	// +optional
	Enabled bool `json:"enabled"`
	// Platforms holds all the platforms which this command supports.
	// +optional
	Platforms PlatformList `json:"platforms,omitempty"`
	// RequiresClone defines if this command wants to clone the repository
	// and thus, requires auth information from the repository (which should have it).
	//
	// required: false
	// TODO: Think about this. Maybe use GitRepository or Flux to reconcile the source so every command has
	// access to it implicitly.
	//RequiresClone bool `json:"requires_clone"`
}

// CommandStatus defines the observed state of Command
type CommandStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Command is the Schema for the commands API
type Command struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommandSpec   `json:"spec,omitempty"`
	Status CommandStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CommandList contains a list of Command
type CommandList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Command `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Command{}, &CommandList{})
}
