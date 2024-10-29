package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StatefulSetReconciler struct {
	client.Client
}

// NewStatefulSetReconciler creates a new StatefulSetReconciler with the given client.
func NewStatefulSetReconciler(client client.Client) *StatefulSetReconciler {
	return &StatefulSetReconciler{
		Client: client,
	}
}

// Reconcile ensures that the desired StatefulSet is present in the cluster.
// If the StatefulSet does not exist, it will be created. If it exists, it will be updated if shouldUpdate is true.
func (r *StatefulSetReconciler) Reconcile(ctx context.Context, desiredSts *appsv1.StatefulSet) error {
	return r.ReconcileWithUpdates(ctx, desiredSts)
}

// ReconcileWithUpdates ensures that the desired StatefulSet is present in the cluster.
// If the StatefulSet does not exist, it will be created. If it exists, it will be updated based on the shouldUpdate flag.
func (r *StatefulSetReconciler) ReconcileWithUpdates(ctx context.Context, desiredSts *appsv1.StatefulSet) error {
	key := client.ObjectKeyFromObject(desiredSts)
	var existingSts appsv1.StatefulSet
	if err := r.Get(ctx, key, &existingSts); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting StatefulSet: %v", err)
		}

		if err := r.Create(ctx, desiredSts); err != nil {
			return fmt.Errorf("error creating StatefulSet: %v", err)
		}
		return nil
	}

	// NOTE: should we implement logic to only patch depending on UpdateStrategy?
	patch := client.MergeFrom(existingSts.DeepCopy())
	existingSts.Spec.Template = desiredSts.Spec.Template
	existingSts.Spec.UpdateStrategy = desiredSts.Spec.UpdateStrategy
	existingSts.Spec.Replicas = desiredSts.Spec.Replicas
	return r.Patch(ctx, &existingSts, patch)
}
