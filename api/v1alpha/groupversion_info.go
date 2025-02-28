// Package v1 contains API Schema definitions for the operator v1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.conduit.io
package v1alpha

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersionKind is group version used to register these objects
	GroupVersionKind = schema.GroupVersionKind{
		Group:   "operator.conduit.io",
		Version: "v1alpha",
		Kind:    "Conduit",
	}

	// GroupKind is the group kind
	GroupKind = GroupVersionKind.GroupKind()

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersionKind.GroupVersion()}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
