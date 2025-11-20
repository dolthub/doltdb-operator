// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GrantSpec defines the desired state of Grant
type GrantSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	SQLTemplate `json:",inline"`
	// DoltDBRef is a reference to a DoltDB object.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// Privileges to use in the Grant.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Privileges []string `json:"privileges"`
	// Database to use in the Grant.
	// +optional
	// +kubebuilder:default=*
	Database string `json:"database,omitempty"`
	// Table to use in the Grant.
	// +optional
	// +kubebuilder:default=*
	Table string `json:"table,omitempty"`
	// Username to use in the Grant.
	// +kubebuilder:validation:Required
	Username string `json:"username"`
	// Host to use in the Grant. It can be localhost, an IP or '%'.
	// +optional
	// +kubebuilder:MaxLength=255
	Host *string `json:"host,omitempty"`
	// GrantOption to use in the Grant.
	// +optional
	// +kubebuilder:default=false
	GrantOption bool `json:"grantOption,omitempty"`
}

// GrantStatus defines the observed state of Grant
type GrantStatus struct {
	// Conditions for the Grant object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (g *GrantStatus) SetCondition(condition metav1.Condition) {
	if g.Conditions == nil {
		g.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&g.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="Database",type="string",JSONPath=".spec.database"
// +kubebuilder:printcolumn:name="Table",type="string",JSONPath=".spec.table"
// +kubebuilder:printcolumn:name="Username",type="string",JSONPath=".spec.username"
// +kubebuilder:printcolumn:name="GrantOpt",type="string",JSONPath=".spec.grantOption"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Grant is the Schema for the grants API
type Grant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrantSpec   `json:"spec,omitempty"`
	Status GrantStatus `json:"status,omitempty"`
}

func (g *Grant) IsBeingDeleted() bool {
	return !g.DeletionTimestamp.IsZero()
}

func (m *Grant) IsReady() bool {
	return meta.IsStatusConditionTrue(m.Status.Conditions, ConditionTypeReady)
}

func (g *Grant) DoltDBRef() *DoltDBRef {
	return &g.Spec.DoltDBRef
}

func (d *Grant) RequeueInterval() *metav1.Duration {
	return d.Spec.RequeueInterval
}

func (g *Grant) RetryInterval() *metav1.Duration {
	return g.Spec.RetryInterval
}

func (g *Grant) CleanupPolicy() *CleanupPolicy {
	return g.Spec.CleanupPolicy
}

func (g *Grant) AccountName() string {
	return fmt.Sprintf("'%s'@'%s'", g.Spec.Username, g.HostnameOrDefault())
}

func (g *Grant) HostnameOrDefault() string {
	if g.Spec.Host != nil && *g.Spec.Host != "" {
		return *g.Spec.Host
	}
	return "%"
}

// GrantUsernameFieldPath is the path related to the username field.
const GrantUsernameFieldPath = ".spec.username"

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (g *Grant) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case GrantUsernameFieldPath:
		return func(obj client.Object) []string {
			grant, ok := obj.(*Grant)
			if !ok {
				return nil
			}
			if grant.Spec.Username != "" {
				return []string{grant.Spec.Username}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// +kubebuilder:object:root=true

// GrantList contains a list of Grant
type GrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Grant `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *GrantList) ListItems() []ctrlclient.Object {
	items := make([]ctrlclient.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&Grant{}, &GrantList{})
}
