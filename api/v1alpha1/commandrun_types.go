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

// EventRef is a reference to an event.
type EventRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// CommandRunSpec defines the desired state of CommandRun
type CommandRunSpec struct {
	// EventRef is the ref of the event that this run belongs to.
	EventRef EventRef `json:"eventRef"`
	// CommandName is the name of the command that is being executed.
	CommandName string `json:"commandName"`
}

// CommandRunStatus defines the observed state of CommandRun
type CommandRunStatus struct {
	// Status is the current state of the command run.
	// example: created, running, failed, success
	Status string `json:"status"`
	// Outcome is any output of the command. Stdout and stderr combined.
	// +optional
	Outcome string `json:"outcome,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CommandRun is the Schema for the commandruns API
type CommandRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CommandRunSpec   `json:"spec,omitempty"`
	Status CommandRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CommandRunList contains a list of CommandRun
type CommandRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CommandRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CommandRun{}, &CommandRunList{})
}
