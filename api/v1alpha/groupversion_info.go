// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

// Package v1alpha contains API Schema definitions for the dolt v1alpha API group
// +kubebuilder:object:generate=true
// +groupName=k8s.dolthub.com

package v1alpha

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "k8s.dolthub.com", Version: "v1alpha"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
