// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

// CronJobKey returns the NamespacedName for the BackupSchedule CronJob.
func (bs *BackupSchedule) CronJobKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-backup-schedule", bs.Name),
		Namespace: bs.Namespace,
	}
}
