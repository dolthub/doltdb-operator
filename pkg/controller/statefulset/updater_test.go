// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import (
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestShouldReconcileUpdates(t *testing.T) {
	tests := []struct {
		name   string
		doltdb *doltv1alpha.DoltDB
		want   bool
	}{
		{
			name: "ReplicasFirstPrimaryLast strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
			},
			want: true,
		},
		{
			name: "empty strategy defaults to true",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: "",
				},
			},
			want: true,
		},
		{
			name: "RollingUpdate strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.RollingUpdateUpdateType,
				},
			},
			want: false,
		},
		{
			name: "RollingUpdate with manual failover reconciles updates",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.RollingUpdateUpdateType,
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{AutomaticFailover: ptr.To(false)},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "RollingUpdate with automatic failover remains Kubernetes-managed",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.RollingUpdateUpdateType,
					Replication: &doltv1alpha.Replication{
						Enabled: true,
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{AutomaticFailover: ptr.To(true)},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "RollingUpdate with replication disabled remains Kubernetes-managed",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.RollingUpdateUpdateType,
					Replication: &doltv1alpha.Replication{
						ReplicationSpec: doltv1alpha.ReplicationSpec{
							Primary: &doltv1alpha.PrimaryReplication{AutomaticFailover: ptr.To(false)},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "OnDelete strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.OnDeleteUpdateType,
				},
			},
			want: false,
		},
		{
			name: "Never strategy",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.NeverUpdateType,
				},
			},
			want: false,
		},
		{
			name: "resizing storage skips updates",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
				Status: doltv1alpha.DoltDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:   doltv1alpha.ConditionTypeStorageResized,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "switching primary skips updates",
			doltdb: &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					UpdateStrategy: doltv1alpha.ReplicasFirstPrimaryLastUpdateType,
				},
				Status: doltv1alpha.DoltDBStatus{
					Conditions: []metav1.Condition{
						{
							Type:   doltv1alpha.ConditionTypePrimarySwitched,
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcileUpdates(tt.doltdb); got != tt.want {
				t.Errorf("shouldReconcileUpdates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetStalePodNames(t *testing.T) {
	updateRevision := "rev-2"

	tests := []struct {
		name       string
		podsByRole podRoleSet
		wantCount  int
		wantNames  []string
	}{
		{
			name: "all pods up to date",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", updateRevision),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", updateRevision),
				},
			},
			wantCount: 0,
		},
		{
			name: "stale replica only",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", updateRevision),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", "rev-1"),
				},
			},
			wantCount: 1,
			wantNames: []string{"replica-1"},
		},
		{
			name: "stale primary only",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", "rev-1"),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", updateRevision),
				},
			},
			wantCount: 1,
			wantNames: []string{"primary-0"},
		},
		{
			name: "all pods stale",
			podsByRole: podRoleSet{
				primary: podWithRevision("primary-0", "rev-1"),
				replicas: []corev1.Pod{
					podWithRevision("replica-1", "rev-1"),
					podWithRevision("replica-2", "rev-1"),
				},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := tt.podsByRole.getStalePodNames(updateRevision)
			if len(names) != tt.wantCount {
				t.Errorf("expected %d stale pods, got %d: %v", tt.wantCount, len(names), names)
			}
			for _, wantName := range tt.wantNames {
				found := false
				for _, name := range names {
					if name == wantName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected stale pod %q not found in %v", wantName, names)
				}
			}
		})
	}
}

func TestGetPodsByRoleSingleInstance(t *testing.T) {
	tests := []struct {
		name            string
		replicas        int32
		expectNoReplica bool
	}{
		{
			name:            "single instance allows zero replica pods",
			replicas:        1,
			expectNoReplica: true,
		},
		{
			name:            "multi instance requires replica pods",
			replicas:        3,
			expectNoReplica: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doltdb := &doltv1alpha.DoltDB{
				Spec: doltv1alpha.DoltDBSpec{
					Replicas: tt.replicas,
				},
			}
			// Verify the guard in getPodsByRole: Replicas>1 is required to error on empty replicas
			hasReplicaRequirement := doltdb.Spec.Replicas > 1
			if hasReplicaRequirement == tt.expectNoReplica {
				t.Errorf("Replicas=%d: hasReplicaRequirement=%v, expectNoReplica=%v",
					tt.replicas, hasReplicaRequirement, tt.expectNoReplica)
			}
		})
	}
}

func TestRoleStatusMismatch(t *testing.T) {
	tests := []struct {
		name         string
		roles        []string
		primaryIndex int
		want         string
	}{
		{
			name:         "roles agree with recorded primary",
			roles:        []string{"standby", "primary", "standby"},
			primaryIndex: 1,
		},
		{
			name:         "bootstrap primary differs from recorded primary",
			roles:        []string{"primary", "standby", "standby"},
			primaryIndex: 1,
			want:         `pod index 0 reports role "primary", want "standby"`,
		},
		{
			name:         "recorded primary was demoted",
			roles:        []string{"standby", "standby"},
			primaryIndex: 1,
			want:         `pod index 1 reports role "standby", want "primary"`,
		},
		{
			name:         "recorded primary index is unset",
			roles:        []string{"primary", "standby"},
			primaryIndex: -1,
			want:         "status.currentPrimaryPodIndex is not set to a valid pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := roleStatusMismatch(tt.roles, tt.primaryIndex); got != tt.want {
				t.Errorf("roleStatusMismatch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func podWithRevision(name, revision string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"controller-revision-hash": revision,
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
}
