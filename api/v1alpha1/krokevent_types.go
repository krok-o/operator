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

// CommandTemplate contains command specifications.
type CommandTemplate struct {
	Spec KrokCommandSpec `json:"spec"`
	Name string          `json:"name"`
}

// KrokEventSpec defines the desired state of KrokEvent
type KrokEventSpec struct {
	// Payload is the received event payload from the provider.
	Payload string `json:"payload"`
	// Type defines the event type such as: push, pull, ping...
	Type string `json:"type"`
	// CommandsToRun contains a list of commands that this event needs to execute.
	CommandsToRun []CommandTemplate `json:"commandsToRun"`
}

// GetCommandsToRun returns a list of commands that needs to be executed.
func (in KrokEvent) GetCommandsToRun() []CommandTemplate {
	result := make([]CommandTemplate, 0)
	for _, cmd := range in.Spec.CommandsToRun {
		if in.Status.FailedCommands.Has(cmd.Name) || in.Status.SucceededCommands.Has(cmd.Name) {
			continue
		}

		result = append(result, cmd)
	}

	return result
}

// Command contains details about the outcome of a job and the name.
type Command struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
}

type CommandList []Command

func (c CommandList) Has(name string) bool {
	for _, cmd := range c {
		if cmd.Name == name {
			return true
		}
	}

	return false
}

// KrokEventStatus defines the observed state of KrokEvent
type KrokEventStatus struct {
	// FailedJobs contains command runs which failed for a given event.
	// +optional
	FailedCommands CommandList `json:"failedCommands,omitempty"`
	// SucceededJobs contains command runs which succeeded for a given event.
	// +optional
	SucceededCommands CommandList `json:"succeededCommands,omitempty"`
	// RunningCommands contains commands which are currently in-progress.
	// +optional
	RunningCommands map[string]bool `json:"runningCommands,omitempty"`

	// ObservedGeneration is the last reconciled generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the status conditions of the object.
func (in KrokEvent) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *KrokEvent) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetStatusConditions returns a pointer to the Status.Conditions slice.
// Deprecated: use GetConditions instead.
func (in *KrokEvent) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// KrokEvent is the Schema for the krokevents API
type KrokEvent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KrokEventSpec   `json:"spec,omitempty"`
	Status KrokEventStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KrokEventList contains a list of KrokEvent
type KrokEventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KrokEvent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KrokEvent{}, &KrokEventList{})
}
