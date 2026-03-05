// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package conditions

import (
	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetBackupPending sets the Ready condition to False with a BackupPending reason.
func SetBackupPending(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonBackupPending,
		Message: "Backup pending",
	})
}

// SetBackupRunning sets the Ready condition to False with a BackupRunning reason.
func SetBackupRunning(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonBackupRunning,
		Message: "Backup running",
	})
}

// SetBackupCompleted sets the Ready and BackupCompleted conditions to True.
func SetBackupCompleted(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonBackupCompleted,
		Message: "Backup completed",
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeBackupCompleted,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonBackupCompleted,
		Message: "Backup completed",
	})
}

// SetBackupFailed sets the Ready condition to False and BackupCompleted to False
// with a BackupFailed reason.
func SetBackupFailed(c Conditioner, message string) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonBackupFailed,
		Message: message,
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeBackupCompleted,
		Status:  metav1.ConditionFalse,
		Reason:  doltv1alpha.ConditionReasonBackupFailed,
		Message: message,
	})
}

// SetBackupScheduleCreated sets the BackupScheduleCreated condition to True.
func SetBackupScheduleCreated(c Conditioner) {
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonBackupScheduleCreated,
		Message: "Backup schedule active",
	})
	c.SetCondition(metav1.Condition{
		Type:    doltv1alpha.ConditionTypeBackupScheduleCreated,
		Status:  metav1.ConditionTrue,
		Reason:  doltv1alpha.ConditionReasonBackupScheduleCreated,
		Message: "Backup schedule created",
	})
}
