// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package patch

import (
	"context"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PatcherDoltDB func(*doltv1alpha.DoltDBStatus) error

func PatchStatus(ctx context.Context, r client.Client, doltdb *doltv1alpha.DoltDB, patcher PatcherDoltDB) error {
	patch := client.MergeFrom(doltdb.DeepCopy())
	if err := patcher(&doltdb.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, doltdb, patch)
}

type PatcherSnapshot func(*doltv1alpha.SnapshotStatus) error

func PatchSnapshotStatus(ctx context.Context, r client.Client, snapshot *doltv1alpha.Snapshot, patcher PatcherSnapshot) error {
	patch := client.MergeFrom(snapshot.DeepCopy())
	if err := patcher(&snapshot.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, snapshot, patch)
}

type PatcherBackup func(*doltv1alpha.BackupStatus) error

func PatchBackupStatus(
	ctx context.Context,
	r client.Client,
	backup *doltv1alpha.Backup,
	patcher PatcherBackup,
) error {
	patch := client.MergeFrom(backup.DeepCopy())
	if err := patcher(&backup.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, backup, patch)
}

type PatcherBackupSchedule func(*doltv1alpha.BackupScheduleStatus) error

func PatchBackupScheduleStatus(
	ctx context.Context,
	r client.Client,
	bs *doltv1alpha.BackupSchedule,
	patcher PatcherBackupSchedule,
) error {
	patch := client.MergeFrom(bs.DeepCopy())
	if err := patcher(&bs.Status); err != nil {
		return err
	}
	return r.Status().Patch(ctx, bs, patch)
}
