package v1alpha1

// ResourceRef is a reference to a resource in the cluster.
type ResourceRef struct {
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	//+kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}
