// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupScheduleSpec defines the desired state of BackupSchedule.
//
// +kubebuilder:validation:XValidation:rule="!has(self.databases) || self.databases.all(d, d.matches('^[a-zA-Z0-9_]+$'))",message="bad name"
// +kubebuilder:validation:XValidation:rule="!has(self.databases) || self.databases.all(d, size(d) <= 80)",message="name too long"
type BackupScheduleSpec struct {
	// DoltDBRef is a reference to the DoltDB cluster to back up.
	// +kubebuilder:validation:Required
	DoltDBRef DoltDBRef `json:"doltDBRef"`
	// Storage defines where backups will be stored.
	// +kubebuilder:validation:Required
	Storage BackupStorage `json:"storage"`
	// Schedule is the cron expression defining when backups should be created.
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`
	// Databases is the list of databases to back up. If empty, all databases are backed up.
	// +optional
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=80
	Databases []string `json:"databases,omitempty"`
	// Suspend indicates whether the schedule is suspended.
	// +optional
	Suspend *bool `json:"suspend,omitempty"`
	// BackoffLimit specifies the number of retries for each created Backup.
	// +kubebuilder:default:=2
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// Resources defines the compute resources for backup jobs.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// ImagePullSecrets specifies the secrets to use for pulling container images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// BackupScheduleStatus defines the observed state of BackupSchedule.
type BackupScheduleStatus struct {
	// Conditions for the BackupSchedule object.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// LastScheduleTime is the last time a Backup was created by this schedule.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`
	// NextScheduleTime is the next time a Backup will be created by this schedule.
	// +optional
	NextScheduleTime *metav1.Time `json:"nextScheduleTime,omitempty"`
	// LastBackupRef is the name of the last Backup created by this schedule.
	// +optional
	LastBackupRef string `json:"lastBackupRef,omitempty"`
}

// SetCondition sets or updates a status condition on the BackupSchedule.
func (in *BackupScheduleStatus) SetCondition(condition metav1.Condition) {
	if in.Conditions == nil {
		in.Conditions = make([]metav1.Condition, 0)
	}
	meta.SetStatusCondition(&in.Conditions, condition)
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type="string",JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Suspend",type="boolean",JSONPath=".spec.suspend"
// +kubebuilder:printcolumn:name="DoltDB",type="string",JSONPath=".spec.doltDBRef.name"
// +kubebuilder:printcolumn:name="Last Schedule",type="date",JSONPath=".status.lastScheduleTime"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BackupSchedule is the Schema for the backupschedules API.
type BackupSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupScheduleSpec   `json:"spec,omitempty"`
	Status BackupScheduleStatus `json:"status,omitempty"`
}

// DoltDBRef returns a pointer to the DoltDB reference.
func (bs *BackupSchedule) DoltDBRef() *DoltDBRef {
	return &bs.Spec.DoltDBRef
}

// IsReady indicates whether the BackupSchedule is in a ready state.
func (bs *BackupSchedule) IsReady() bool {
	return meta.IsStatusConditionTrue(bs.Status.Conditions, ConditionTypeReady)
}

// IsSuspended returns true if the schedule is suspended.
func (bs *BackupSchedule) IsSuspended() bool {
	return bs.Spec.Suspend != nil && *bs.Spec.Suspend
}

// GetBackoffLimit returns the backoff limit or the default.
func (bs *BackupSchedule) GetBackoffLimit() int32 {
	if bs.Spec.BackoffLimit != nil {
		return *bs.Spec.BackoffLimit
	}
	return DefaultBackoffLimit
}

// +kubebuilder:object:root=true

// BackupScheduleList contains a list of BackupSchedule.
type BackupScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupSchedule{}, &BackupScheduleList{})
}
