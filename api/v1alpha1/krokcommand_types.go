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

// KrokCommandSpec defines the desired state of KrokCommand
type KrokCommandSpec struct {
	// ReadInputFromSecret if defined, the command will take a list of key/value pairs in a secret
	// and apply them as arguments to the command.
	ReadInputFromSecret *Ref `json:"readInputFromSecret,omitempty"`
	// CommandHasOutputToWrite if defined, it signals the underlying Job, to put its output into a generated
	// and created secret.
	CommandHasOutputToWrite bool `json:"commandHasOutputToWrite,omitempty"`
	// Schedule of the command.
	// example: 0 * * * * // follows cron job syntax.
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// Image defines the image name and tag of the command
	// example: krok-hook/slack-notification:v0.0.1
	Image string `json:"image"`
	// Enabled defines if this command can be executed or not.
	// +optional
	Enabled bool `json:"enabled"`
	// Platforms holds all the platforms which this command supports.
	// +optional
	Platforms []string `json:"platforms,omitempty"`
	// Dependencies defines a list of command names that this command depends on.
	Dependencies []string `json:"dependencies,omitempty"`
}

// KrokCommandStatus defines the observed state of KrokCommand
type KrokCommandStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KrokCommand is the Schema for the krokcommands API
type KrokCommand struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KrokCommandSpec   `json:"spec,omitempty"`
	Status KrokCommandStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KrokCommandList contains a list of KrokCommand
type KrokCommandList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KrokCommand `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KrokCommand{}, &KrokCommandList{})
}
