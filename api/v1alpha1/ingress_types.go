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

// IngressSpec defines the desired state of Ingress
type IngressSpec struct {
	// ProviderRef is a reference to the provider that should be used to update the IP.
	// This must match a provider object in the same namespace.
	// +kubebuilder:validation:Required
	ProviderRef ResourceRef `json:"providerRef"`

	// Notifiers is a list of notifiers that the provider should use to notify for changes.
	// +kubebuilder:validation:Optional
	NotifierRefs []ResourceRef `json:"notifierRefs,omitempty"`
}

// IngressStatus defines the observed state of Ingress
type IngressStatus struct {
	// ProviderIP is the IP address that the provider has set for the Ingress.
	ProviderIP string `json:"providerIP,omitempty"`

	// PublicIP is your public IP address.
	PublicIP string `json:"publicIP,omitempty"`

	// ObservedGeneration is the most recent generation observed for this Ingress.
	// This gets updated at the end of a successful reconciliation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Provider.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Ingress is the Schema for the ingresses API
type Ingress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IngressSpec   `json:"spec,omitempty"`
	Status IngressStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IngressList contains a list of Ingress
type IngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ingress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ingress{}, &IngressList{})
}
