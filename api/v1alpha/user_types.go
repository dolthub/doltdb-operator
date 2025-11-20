// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	SQLTemplate `json:",inline"`
	// DoltDBRef is a reference to a DoltDB object.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// PasswordSecretKeyRef is a reference to the password to be used by the User.
	// If not provided, the account will be locked and the password will expire.
	// If the referred Secret is labeled with "k8s.dolthub.com/watch", updates may
	// be performed to the Secret in order to update the password.
	// +kubebuilder:validation:Required
	PasswordSecretKeyRef *SecretKeySelector `json:"passwordSecretKeyRef,omitempty"`
	// Name overrides the default name provided by metadata.name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=80
	Name string `json:"name,omitempty"`
	// Host related to the User.
	// +optional
	// +kubebuilder:validation:MaxLength=255
	Host string `json:"host,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// Conditions for the User object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (u *UserStatus) SetCondition(condition metav1.Condition) {
	if u.Conditions == nil {
		u.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&u.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

func (u *User) AccountName() string {
	return fmt.Sprintf("'%s'@'%s'", u.Username(), u.HostnameOrDefault())
}

func (u *User) Username() string {
	return u.Spec.Name
}

func (u *User) HostnameOrDefault() string {
	if u.Spec.Host != "" {
		return u.Spec.Host
	}
	return "%"
}

func (u *User) IsBeingDeleted() bool {
	return !u.DeletionTimestamp.IsZero()
}

func (u *User) IsReady() bool {
	return meta.IsStatusConditionTrue(u.Status.Conditions, ConditionTypeReady)
}

func (u *User) DoltDBRef() *DoltDBRef {
	return &u.Spec.DoltDBRef
}

func (d *User) RequeueInterval() *metav1.Duration {
	return d.Spec.RequeueInterval
}

func (u *User) RetryInterval() *metav1.Duration {
	return u.Spec.RetryInterval
}

func (u *User) CleanupPolicy() *CleanupPolicy {
	return u.Spec.CleanupPolicy
}

const (
	// UserPasswordSecretFieldPath is the path related to the password Secret field.
	UserPasswordSecretFieldPath = ".spec.passwordSecretKeyRef.name"
)

// IndexerFuncForFieldPath returns an indexer function for a given field path.
func (u *User) IndexerFuncForFieldPath(fieldPath string) (client.IndexerFunc, error) {
	switch fieldPath {
	case UserPasswordSecretFieldPath:
		return func(obj client.Object) []string {
			user, ok := obj.(*User)
			if !ok {
				return nil
			}
			if user.Spec.PasswordSecretKeyRef != nil && user.Spec.PasswordSecretKeyRef.LocalObjectReference.Name != "" {
				return []string{user.Spec.PasswordSecretKeyRef.LocalObjectReference.Name}
			}
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unsupported field path: %s", fieldPath)
	}
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// ListItems gets a copy of the Items slice.
func (m *UserList) ListItems() []client.Object {
	items := make([]client.Object, len(m.Items))
	for i, item := range m.Items {
		items[i] = item.DeepCopy()
	}
	return items
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
