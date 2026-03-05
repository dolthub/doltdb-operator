// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package controller

import (
	"context"
	"fmt"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/controller/backupschedule"
	"github.com/electronicarts/doltdb-operator/pkg/controller/database"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BackupScheduleReconciler reconciles a BackupSchedule object.
type BackupScheduleReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	RefResolver        *refresolver.RefResolver
	ScheduleReconciler *backupschedule.Reconciler
}

// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backupschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.dolthub.com,resources=backupschedules/finalizers,verbs=update

func (r *BackupScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("backupschedule", req.NamespacedName)

	var bs doltv1alpha.BackupSchedule
	if err := r.Get(ctx, req.NamespacedName, &bs); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.Info("Running reconciler for BackupSchedule")

	if bs.IsSuspended() {
		logger.Info("BackupSchedule is suspended, skipping")
		return ctrl.Result{}, nil
	}

	// Resolve DoltDB and wait for readiness
	doltdb, err := r.RefResolver.DoltDB(ctx, bs.DoltDBRef(), bs.GetNamespace())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error resolving DoltDB: %w", err)
	}
	result, err := database.WaitForDoltDB(ctx, r.Client, doltdb, false)
	if err != nil || !result.IsZero() {
		return result, err
	}

	return r.ScheduleReconciler.Reconcile(ctx, &bs)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&doltv1alpha.BackupSchedule{}).
		Complete(r)
}
