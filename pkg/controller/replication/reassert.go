// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"context"
	"errors"
	"fmt"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/conditions"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	doltsql "github.com/electronicarts/doltdb-operator/pkg/dolt/sql"
	"github.com/electronicarts/doltdb-operator/pkg/statefulset"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type reassertAction int

const (
	reassertNone reassertAction = iota
	reassertPrimaryPresent
	reassertPromote
	reassertBlocked
)

type reassertDecision struct {
	action  reassertAction
	index   int
	epoch   int
	message string
}

func (r *ReplicationReconciler) reconcilePrimaryReassert(
	ctx context.Context,
	req *reconcileRequest,
	logger logr.Logger,
) (ctrl.Result, error) {
	if !req.doltdb.IsManualFailoverReplication() || req.doltdb.IsResizingStorage() {
		return ctrl.Result{}, nil
	}

	states := GetDBStates(ctx, req.doltdb, req.clientSet)
	candidate := ptr.Deref(req.doltdb.Status.CurrentPrimaryPodIndex, -1)

	return r.applyReassertDecision(ctx, req, decideReassert(req.doltdb, states, candidate), logger)
}

// applyReassertDecision carries out a reassert decision: conditions, events and, when proven safe, the promotion.
func (r *ReplicationReconciler) applyReassertDecision(
	ctx context.Context,
	req *reconcileRequest,
	decision reassertDecision,
	logger logr.Logger,
) (ctrl.Result, error) {
	switch decision.action {
	case reassertNone:
		logger.Info("Pod replication state unavailable; retrying primary check", "error", decision.message)
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	case reassertPrimaryPresent:
		if meta.IsStatusConditionTrue(req.doltdb.Status.Conditions, doltv1alpha.ConditionTypePrimaryAvailable) {
			return ctrl.Result{}, nil
		}
		if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltDBStatus) {
			conditions.SetPrimaryAvailable(status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting primary-available condition: %w", err)
		}

		return ctrl.Result{}, nil
	case reassertBlocked:
		current := meta.FindStatusCondition(req.doltdb.Status.Conditions, doltv1alpha.ConditionTypePrimaryAvailable)
		if current != nil && current.Status == metav1.ConditionFalse && current.Message == decision.message {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		r.recorder.Event(req.doltdb, corev1.EventTypeWarning, doltv1alpha.ReasonNoPrimary, decision.message)
		if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltDBStatus) {
			conditions.SetNoPrimary(status, decision.message)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting no-primary condition: %w", err)
		}

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	case reassertPromote:
		client, err := req.clientSet.ClientForIndex(ctx, decision.index)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting client for reasserted primary at index %d: %w", decision.index, err)
		}
		if err := client.AssumeRole(ctx, doltsql.AssumeRoleOpts{Epoch: decision.epoch, Role: dolt.PrimaryRoleValue}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error reasserting primary at index %d: %w", decision.index, err)
		}
		if err := client.Close(); err != nil {
			return ctrl.Result{}, fmt.Errorf("error closing reasserted primary client at index %d: %w", decision.index, err)
		}
		req.clientSet.RemoveClientFromCache(decision.index)

		r.recorder.Event(req.doltdb, corev1.EventTypeWarning, doltv1alpha.ReasonPrimaryReasserted, decision.message)
		if err := r.patchStatus(ctx, req.doltdb, func(status *doltv1alpha.DoltDBStatus) {
			status.UpdateReplicationEpoch(decision.epoch)
			conditions.SetPrimaryAvailable(status)
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error recording reasserted primary: %w", err)
		}

		logger.Info("Reasserted the last primary", "pod-index", decision.index, "epoch", decision.epoch)

		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, fmt.Errorf("unknown primary reassert decision %d", decision.action)
	}
}

func decideReassert(doltdb *doltv1alpha.DoltDB, states []dolt.DBState, candidate int) reassertDecision {
	stateErrs := make([]error, 0, len(states))
	for _, state := range states {
		stateErrs = append(stateErrs, state.Err)
	}
	if joined := errors.Join(stateErrs...); joined != nil {
		// An unreadable pod cannot prove the cluster has no primary, so never act on a partial read.
		return reassertDecision{action: reassertNone, message: joined.Error()}
	}

	primaryCount := 0
	for _, state := range states {
		if state.Role == dolt.PrimaryRoleValue.String() {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return reassertDecision{
			action:  reassertBlocked,
			message: "more than one pod reports role primary",
		}
	}
	if primaryCount == 1 {
		return reassertDecision{action: reassertPrimaryPresent}
	}

	for i, state := range states {
		if state.Role != dolt.StandbyRoleValue.String() {
			return reassertDecision{
				action:  reassertBlocked,
				message: fmt.Sprintf("pod %s reports role %q", statefulset.PodName(doltdb.ObjectMeta, i), state.Role),
			}
		}
	}

	if candidate < 0 || candidate >= len(states) {
		return reassertDecision{
			action:  reassertBlocked,
			message: "status.currentPrimaryPodIndex is not set to a valid pod",
		}
	}

	candidateState := states[candidate]
	for i, state := range states {
		if i != candidate && candidateState.Epoch <= state.Epoch {
			return reassertDecision{
				action: reassertBlocked,
				message: fmt.Sprintf(
					"all pods are standby; %s (epoch %d) is not uniquely the highest epoch, refusing automatic promotion",
					statefulset.PodName(doltdb.ObjectMeta, candidate), candidateState.Epoch,
				),
			}
		}
	}

	epoch := candidateState.Epoch + 1

	return reassertDecision{
		action: reassertPromote,
		index:  candidate,
		epoch:  epoch,
		message: fmt.Sprintf(
			"all pods were standby; reasserted %s as primary at epoch %d",
			statefulset.PodName(doltdb.ObjectMeta, candidate),
			epoch,
		),
	}
}
