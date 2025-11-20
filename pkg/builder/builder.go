// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package builder

import "k8s.io/apimachinery/pkg/runtime"

// Builder is a struct that holds a Kubernetes runtime scheme.
type Builder struct {
	scheme *runtime.Scheme
}

// NewBuilder creates a new instance of Builder with the provided runtime scheme.
func NewBuilder(scheme *runtime.Scheme) *Builder {
	return &Builder{
		scheme: scheme,
	}
}
