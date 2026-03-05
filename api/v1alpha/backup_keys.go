// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package v1alpha

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

// BackupJobKey returns the NamespacedName for the backup Job.
func (b *Backup) BackupJobKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-backup", b.Name),
		Namespace: b.Namespace,
	}
}
