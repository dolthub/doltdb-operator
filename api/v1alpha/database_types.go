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

package v1alpha

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// SQLTemplate defines templates to configure SQL objects.
	SQLTemplate `json:",inline"`
	// DoltDBRef is a reference to a DoltDB object.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern:=`^[a-zA-Z0-9_]+$`
	// +kubebuilder:validation:MaxLength=80
	// Name is the name of the DoltDB database, executed as CREATE DATABASE <name>
	Name *string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="utf8mb4_0900_bin"
	// Collation is the collation of the DoltDB database, executed as CREATE DATABASE <name> COLLATE <collation>
	// doltdb's default collation: https://docs.dolthub.com/sql-reference/sql-support/collations-and-charsets
	Collation string `json:"collation,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="utf8mb4"
	// Chartset is the charset of the DoltDB database, executed as CREATE DATABASE <name> CHARACTER SET <charset>
	CharSet string `json:"charset,omitempty"`
	// +kubebuilder:validation:Optional
	// SystemBranches is the list of system branches to create in the DoltDB database (e.g master, global)
	SystemBranches []string `json:"systemBranches,omitempty"`
	// +kubebuilder:validation:Optional
	// DoltIgnorePatterns is the list of tables to ignore in the DoltDB database
	// Reference: https://www.dolthub.com/blog/2023-05-03-using-dolt_ignore-to-prevent-accidents/
	DoltIgnorePatterns []string `json:"doltIgnorePatterns,omitempty"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	// Conditions for the Database object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (d *DatabaseStatus) SetCondition(condition metav1.Condition) {
	if d.Conditions == nil {
		d.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&d.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message"
// +kubebuilder:printcolumn:name="CharSet",type="string",JSONPath=".spec.characterSet"
// +kubebuilder:printcolumn:name="Collate",type="string",JSONPath=".spec.collate"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".spec.name"

// Database is the Schema for the Dolt Database API
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

func (d *Database) DoltDBRef() *DoltDBRef {
	return &d.Spec.DoltDBRef
}

func (d *Database) Name() string {
	return *d.Spec.Name
}

func (d *Database) IsBeingDeleted() bool {
	return !d.DeletionTimestamp.IsZero()
}

func (d *Database) IsReady() bool {
	return meta.IsStatusConditionTrue(d.Status.Conditions, ConditionTypeReady)
}

func (d *Database) RequeueInterval() *metav1.Duration {
	return d.Spec.RequeueInterval
}

func (d *Database) RetryInterval() *metav1.Duration {
	return d.Spec.RetryInterval
}

func (d *Database) CleanupPolicy() *CleanupPolicy {
	return d.Spec.CleanupPolicy
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
