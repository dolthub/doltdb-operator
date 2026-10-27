// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"context"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/refresolver"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestGetDBStatesRecordsUnreadablePods pins the invariant decideReassert's fail-closed rule depends on:
// a pod whose client cannot even be built must come back with Err set, not as a zero-value DBState
// that would otherwise look like a pod reporting an empty role.
func TestGetDBStatesRecordsUnreadablePods(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, doltv1alpha.AddToScheme(scheme))
	assert.NoError(t, corev1.AddToScheme(scheme))

	doltdb := &doltv1alpha.DoltDB{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"},
		Spec: doltv1alpha.DoltDBSpec{
			Replicas:    2,
			Replication: &doltv1alpha.Replication{Enabled: true},
		},
	}

	// No credentials Secret exists, so ClientForIndex fails for every index without any network access.
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(doltdb).Build()
	clientSet := NewReplicationClientSet(doltdb, refresolver.New(k8sClient))

	states := GetDBStates(context.Background(), doltdb, clientSet)

	if !assert.Len(t, states, 2) {
		return
	}
	for i, state := range states {
		assert.Error(t, state.Err, "state %d should carry the client error", i)
		assert.Empty(t, state.Role, "state %d should not report a role it never read", i)
		assert.Zero(t, state.Epoch, "state %d should not report an epoch it never read", i)
	}

	// The whole point of populating Err: the reassert decision must refuse to act on these states.
	decision := decideReassert(doltdb, states, 0)
	assert.Equal(t, reassertNone, decision.action)
	assert.NotEmpty(t, decision.message)
}
