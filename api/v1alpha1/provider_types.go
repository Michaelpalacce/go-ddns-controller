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
	"github.com/Michaelpalacce/go-ddns-controller/api/v1alpha1/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProviderSpec defines the desired state of Provider
type ProviderSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the name of the provider we want to create.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum:=Cloudflare
	Name string `json:"name"`

	// SecretName is the name of the secret that holds the provider specific configuration.
	// Each provider has its own configuration that is stored in a secret.
	// Providers:
	// - Cloudflare: The secret should have the following keys:
	//   - apiToken: The Cloudflare API token.
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// ConfigMap is the name of the config map that holds the provider specific configuration.
	// +kubebuilder:validation:Required
	ConfigMap string `json:"configMap"`

	// RetryInterval is the interval in seconds that the provider should wait before retrying to update the IP.
	// Default is 900 seconds (15 minutes).
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=900
	RetryInterval int64 `json:"retryInterval"`

	// CustomIPProvider is the URL of the custom IP provider that should be used to get the IP.
	// If this is set, the provider will use this URL to get the IP FIRST, but will fallback to the rest of the IP providers.
	// +kubebuilder:validation:Optional
	CustomIPProvider string `json:"customIPProvider"`

	// Notifiers is a list of notifiers that the provider should use to notify for changes.
	// +kubebuilder:validation:Optional
	NotifierRefs []ResourceRef `json:"notifierRefs,omitempty"`
}

// ProviderStatus defines the observed state of Provider
type ProviderStatus struct {
	// ProviderIP is the IP address that the provider has set.
	ProviderIP string `json:"providerIP,omitempty"`

	// PublicIP is your public IP address.
	PublicIP string `json:"publicIP,omitempty"`

	// ObservedGeneration is the most recent generation observed for this Provider.
	// This gets updated at the end of a successful reconciliation.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the observations of a Provider's current state.
	// Provider.status.conditions.type are: "Available" and "Progressing"
	// Provider.status.conditions.status are one of True, False, Unknown.
	// Provider.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Provider.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

type ProviderCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Provider is the Schema for the providers API
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderSpec   `json:"spec,omitempty"`
	Status ProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderList contains a list of Provider
type ProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Provider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Provider{}, &ProviderList{})
}

// =================================================== Status ===================================================

const (
	ProviderConditionTypeClient = "Client"

	ProviderConditionTypeConfigMap = "ConfigMap"

	ProviderConditionTypeSecret = "Secret"
)

func (p *Provider) Conditions() *conditions.Conditions {
	return &conditions.Conditions{
		Conditions:     &p.Status.Conditions,
		ConditionTypes: []string{ProviderConditionTypeClient, ProviderConditionTypeConfigMap, ProviderConditionTypeSecret},
	}
}
