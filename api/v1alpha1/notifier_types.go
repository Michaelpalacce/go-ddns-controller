/*
Copyright 2024.

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

// NotifierSpec defines the desired state of Notifier
type NotifierSpec struct {
	// Name is the name of the notifier we want to create.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=Webhook
	Name string `json:"name"`

	// SecretName is the name of the secret that holds the notifier specific configuration.
	// Each notifier has its own configuration that is stored in a secret.
	// Notifiers:
	// - Webhook: The secret should have the following keys:
	//   - url: .The Webhook URL. Treated as a secret as it may contain sensitive data.
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// ConfigMap is the name of the config map that holds the provider specific configuration.
	// +kubebuilder:validation:Required
	ConfigMap string `json:"configMap"`
}

// NotifierStatus defines the observed state of Notifier
type NotifierStatus struct {
	// IsReady is the status of the notifier.
	// It is set to true when the notifier is ready to send notifications.
	IsReady bool `json:"isReady,omitempty"`

	// ObservedGeneration is the most recent generation observed for this Notifier.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the observations of a Notifier's current state.
	// Notifier.status.conditions.type are: "Available" and "Progressing"
	// Notifier.status.conditions.status are one of True, False, Unknown.
	// Notifier.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Notifier.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Notifier is the Schema for the notifiers API
type Notifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotifierSpec   `json:"spec,omitempty"`
	Status NotifierStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NotifierList contains a list of Notifier
type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notifier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Notifier{}, &NotifierList{})
}
