// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DoltDBRef is a reference to a DoltDB object.
type DoltDBRef struct {
	// ObjectReference is a reference to a object.
	ObjectReference `json:",inline"`
}

// SQLTemplate defines a template to customize SQL objects.
type SQLTemplate struct {
	// RequeueInterval is used to perform requeue reconciliations.
	// +optional
	RequeueInterval *metav1.Duration `json:"requeueInterval,omitempty"`
	// RetryInterval is the interval used to perform retries.
	// +optional
	RetryInterval *metav1.Duration `json:"retryInterval,omitempty"`
	// CleanupPolicy defines the behavior for cleaning up a SQL resource.
	// +optional
	// +kubebuilder:validation:Enum=Skip;Delete
	CleanupPolicy *CleanupPolicy `json:"cleanupPolicy,omitempty"`
}

// CleanupPolicy defines the behavior for cleaning up a resource.
type CleanupPolicy string

const (
	// CleanupPolicySkip indicates that the resource will NOT be deleted from the database after the CR is deleted.
	CleanupPolicySkip CleanupPolicy = "Skip"
	// CleanupPolicyDelete indicates that the resource will be deleted from the database after the CR is deleted.
	CleanupPolicyDelete CleanupPolicy = "Delete"
)

// Validate returns an error if the CleanupPolicy is not valid.
func (c CleanupPolicy) Validate() error {
	switch c {
	case CleanupPolicySkip, CleanupPolicyDelete:
		return nil
	default:
		return fmt.Errorf("invalid cleanupPolicy: %v", c)
	}
}
