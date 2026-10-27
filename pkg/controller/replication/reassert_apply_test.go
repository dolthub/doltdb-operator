// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"context"
	"testing"
	"time"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestApplyReassertDecision covers the non-promoting branches, which need no Dolt server:
// the conditions written, the events emitted, and the requeue each decision asks for.
// The reassertPromote branch talks SQL to a pod and stays covered by the integration suite.
func TestApplyReassertDecision(t *testing.T) {
	const blockedMessage = "all pods are standby; cluster-1 (epoch 3) is not uniquely the highest epoch, refusing automatic promotion"

	tests := []struct {
		name              string
		conditions        []metav1.Condition
		decision          reassertDecision
		wantResult        ctrl.Result
		wantConditionType bool // whether PrimaryAvailable should exist afterwards
		wantStatus        metav1.ConditionStatus
		wantReason        string
		wantMessage       string
		wantEvents        []string
	}{
		{
			name:       "unreadable state requeues without touching conditions",
			decision:   reassertDecision{action: reassertNone, message: "pod 0 unreachable"},
			wantResult: ctrl.Result{RequeueAfter: 10 * time.Second},
		},
		{
			name:              "a reported primary marks the cluster available",
			decision:          reassertDecision{action: reassertPrimaryPresent},
			wantResult:        ctrl.Result{},
			wantConditionType: true,
			wantStatus:        metav1.ConditionTrue,
			wantReason:        doltv1alpha.ConditionReasonPrimaryAvailable,
			wantMessage:       "A pod reports role primary",
		},
		{
			name: "an already-available cluster is left alone",
			conditions: []metav1.Condition{{
				Type:               doltv1alpha.ConditionTypePrimaryAvailable,
				Status:             metav1.ConditionTrue,
				Reason:             doltv1alpha.ConditionReasonPrimaryAvailable,
				Message:            "A pod reports role primary",
				LastTransitionTime: metav1.Now(),
			}},
			decision:          reassertDecision{action: reassertPrimaryPresent},
			wantResult:        ctrl.Result{},
			wantConditionType: true,
			wantStatus:        metav1.ConditionTrue,
			wantReason:        doltv1alpha.ConditionReasonPrimaryAvailable,
			wantMessage:       "A pod reports role primary",
		},
		{
			name:              "a blocked promotion reports NoPrimary and requeues",
			decision:          reassertDecision{action: reassertBlocked, message: blockedMessage},
			wantResult:        ctrl.Result{RequeueAfter: 10 * time.Second},
			wantConditionType: true,
			wantStatus:        metav1.ConditionFalse,
			wantReason:        doltv1alpha.ConditionReasonNoPrimary,
			wantMessage:       blockedMessage,
			wantEvents:        []string{"Warning " + doltv1alpha.ReasonNoPrimary + " " + blockedMessage},
		},
		{
			name: "an unchanged block does not re-emit its event",
			conditions: []metav1.Condition{{
				Type:               doltv1alpha.ConditionTypePrimaryAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             doltv1alpha.ConditionReasonNoPrimary,
				Message:            blockedMessage,
				LastTransitionTime: metav1.Now(),
			}},
			decision:          reassertDecision{action: reassertBlocked, message: blockedMessage},
			wantResult:        ctrl.Result{RequeueAfter: 10 * time.Second},
			wantConditionType: true,
			wantStatus:        metav1.ConditionFalse,
			wantReason:        doltv1alpha.ConditionReasonNoPrimary,
			wantMessage:       blockedMessage,
		},
		{
			name: "a block for a new reason re-emits its event",
			conditions: []metav1.Condition{{
				Type:               doltv1alpha.ConditionTypePrimaryAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             doltv1alpha.ConditionReasonNoPrimary,
				Message:            blockedMessage,
				LastTransitionTime: metav1.Now(),
			}},
			decision:          reassertDecision{action: reassertBlocked, message: "more than one pod reports role primary"},
			wantResult:        ctrl.Result{RequeueAfter: 10 * time.Second},
			wantConditionType: true,
			wantStatus:        metav1.ConditionFalse,
			wantReason:        doltv1alpha.ConditionReasonNoPrimary,
			wantMessage:       "more than one pod reports role primary",
			wantEvents: []string{
				"Warning " + doltv1alpha.ReasonNoPrimary + " more than one pod reports role primary",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, doltdb, recorder := newReassertTestReconciler(t, tt.conditions)

			result, err := r.applyReassertDecision(
				context.Background(),
				&reconcileRequest{doltdb: doltdb},
				tt.decision,
				logr.Discard(),
			)

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)

			condition := meta.FindStatusCondition(doltdb.Status.Conditions, doltv1alpha.ConditionTypePrimaryAvailable)
			if !tt.wantConditionType {
				assert.Nil(t, condition, "PrimaryAvailable should not be set")
			} else if assert.NotNil(t, condition, "PrimaryAvailable should be set") {
				assert.Equal(t, tt.wantStatus, condition.Status)
				assert.Equal(t, tt.wantReason, condition.Reason)
				assert.Equal(t, tt.wantMessage, condition.Message)
			}

			assert.Equal(t, tt.wantEvents, drainEvents(recorder))
		})
	}
}

func TestApplyReassertDecisionRejectsUnknownAction(t *testing.T) {
	r, doltdb, _ := newReassertTestReconciler(t, nil)

	_, err := r.applyReassertDecision(
		context.Background(),
		&reconcileRequest{doltdb: doltdb},
		reassertDecision{action: reassertAction(99)},
		logr.Discard(),
	)

	assert.ErrorContains(t, err, "unknown primary reassert decision")
}

func newReassertTestReconciler(
	t *testing.T,
	conditions []metav1.Condition,
) (*ReplicationReconciler, *doltv1alpha.DoltDB, *record.FakeRecorder) {
	t.Helper()

	scheme := runtime.NewScheme()
	assert.NoError(t, doltv1alpha.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))

	doltdb := &doltv1alpha.DoltDB{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"},
		Spec: doltv1alpha.DoltDBSpec{
			Replicas: 2,
			Replication: &doltv1alpha.Replication{
				Enabled: true,
				ReplicationSpec: doltv1alpha.ReplicationSpec{
					Primary: &doltv1alpha.PrimaryReplication{
						PodIndex:          ptr.To(1),
						AutomaticFailover: ptr.To(false),
					},
				},
			},
		},
		Status: doltv1alpha.DoltDBStatus{
			CurrentPrimaryPodIndex: ptr.To(1),
			Conditions:             conditions,
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(doltdb).
		WithStatusSubresource(doltdb).
		Build()
	recorder := record.NewFakeRecorder(10)

	return &ReplicationReconciler{Client: k8sClient, recorder: recorder}, doltdb, recorder
}

// drainEvents returns the events recorded so far, or nil when none were emitted.
func drainEvents(recorder *record.FakeRecorder) []string {
	var events []string
	for {
		select {
		case event := <-recorder.Events:
			events = append(events, event)
		default:
			return events
		}
	}
}
