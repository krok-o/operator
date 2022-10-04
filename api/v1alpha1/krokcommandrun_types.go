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

// KrokCommandRunSpec defines the desired state of KrokCommandRun
// The Event which owns this CommandRun will be set as Owner.
type KrokCommandRunSpec struct {
	// CommandName is the name of the command that is being executed.
	CommandName string `json:"commandName"`
}

// KrokCommandRunStatus defines the observed state of KrokCommandRun
type KrokCommandRunStatus struct {
	// Status is the current state of the command run.
	// example: created, running, failed, success
	Status string `json:"status"`
	// Outcome is any output of the command. Stdout and stderr combined.
	// +optional
	Outcome string `json:"outcome,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KrokCommandRun is the Schema for the krokcommandruns API
type KrokCommandRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KrokCommandRunSpec   `json:"spec,omitempty"`
	Status KrokCommandRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KrokCommandRunList contains a list of KrokCommandRun
type KrokCommandRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KrokCommandRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KrokCommandRun{}, &KrokCommandRunList{})
}
