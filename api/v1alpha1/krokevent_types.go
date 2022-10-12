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

// KrokEventSpec defines the desired state of KrokEvent
type KrokEventSpec struct {
	// Payload is the received event payload from the provider.
	Payload string `json:"payload"`
	// Type defines the event type such as: push, pull, ping...
	Type string `json:"type"`
}

// KrokEventStatus defines the observed state of KrokEvent
type KrokEventStatus struct {
	// Done defines if an event is done or not.
	// +optional
	Done bool `json:"done,omitempty"`
	// Outcome holds the outcome of the event such as success, failed...
	// +optional
	Outcome string `json:"outcome,omitempty"`
	// Jobs contains current running jobs for this event.
	// +optional
	Jobs []Ref `json:"jobs,omitempty"`
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
