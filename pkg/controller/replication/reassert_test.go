// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package replication

import (
	"errors"
	"testing"

	doltv1alpha "github.com/electronicarts/doltdb-operator/api/v1alpha"
	"github.com/electronicarts/doltdb-operator/pkg/dolt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDecideReassert(t *testing.T) {
	standby := dolt.StandbyRoleValue.String()
	primary := dolt.PrimaryRoleValue.String()
	doltdb := &doltv1alpha.DoltDB{ObjectMeta: metav1.ObjectMeta{Name: "cluster"}}

	tests := []struct {
		name      string
		states    []dolt.DBState
		candidate int
		want      reassertDecision
	}{
		{
			name:      "reasserts the recorded primary when its epoch is uniquely highest",
			states:    []dolt.DBState{{Role: standby, Epoch: 2}, {Role: standby, Epoch: 3}},
			candidate: 1,
			want: reassertDecision{
				action:  reassertPromote,
				index:   1,
				epoch:   4,
				message: "all pods were standby; reasserted cluster-1 as primary at epoch 4",
			},
		},
		{
			name:      "blocks a recorded primary that is not highest",
			states:    []dolt.DBState{{Role: standby, Epoch: 2}, {Role: standby, Epoch: 3}},
			candidate: 0,
			want: reassertDecision{
				action:  reassertBlocked,
				message: "all pods are standby; cluster-0 (epoch 2) is not uniquely the highest epoch, refusing automatic promotion",
			},
		},
		{
			name:      "blocks a tied highest epoch",
			states:    []dolt.DBState{{Role: standby, Epoch: 3}, {Role: standby, Epoch: 3}},
			candidate: 1,
			want: reassertDecision{
				action:  reassertBlocked,
				message: "all pods are standby; cluster-1 (epoch 3) is not uniquely the highest epoch, refusing automatic promotion",
			},
		},
		{
			name:      "accepts one reported primary",
			states:    []dolt.DBState{{Role: standby, Epoch: 2}, {Role: primary, Epoch: 3}},
			candidate: 1,
			want:      reassertDecision{action: reassertPrimaryPresent},
		},
		{
			name:      "blocks multiple reported primaries",
			states:    []dolt.DBState{{Role: primary, Epoch: 3}, {Role: primary, Epoch: 4}},
			candidate: 1,
			want: reassertDecision{
				action:  reassertBlocked,
				message: "more than one pod reports role primary",
			},
		},
		{
			name: "does not act when any pod state is unavailable",
			states: []dolt.DBState{
				{Role: primary, Epoch: 3},
				{Err: errors.New("state query failed")},
			},
			candidate: 0,
			want:      reassertDecision{action: reassertNone, message: "state query failed"},
		},
		{
			name: "reports every unreadable pod state",
			states: []dolt.DBState{
				{Err: errors.New("pod 0 unreachable")},
				{Err: errors.New("pod 1 unreachable")},
			},
			candidate: 0,
			want:      reassertDecision{action: reassertNone, message: "pod 0 unreachable\npod 1 unreachable"},
		},
		{
			name:      "blocks an unrecognized pod role",
			states:    []dolt.DBState{{Role: "detected_broken_config", Epoch: 2}, {Role: standby, Epoch: 3}},
			candidate: 1,
			want: reassertDecision{
				action:  reassertBlocked,
				message: `pod cluster-0 reports role "detected_broken_config"`,
			},
		},
		{
			name:      "blocks an unset candidate index",
			states:    []dolt.DBState{{Role: standby, Epoch: 2}, {Role: standby, Epoch: 3}},
			candidate: -1,
			want: reassertDecision{
				action:  reassertBlocked,
				message: "status.currentPrimaryPodIndex is not set to a valid pod",
			},
		},
		{
			name:      "blocks an out-of-range candidate index",
			states:    []dolt.DBState{{Role: standby, Epoch: 3}, {Role: standby, Epoch: 2}},
			candidate: 2,
			want: reassertDecision{
				action:  reassertBlocked,
				message: "status.currentPrimaryPodIndex is not set to a valid pod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideReassert(doltdb, tt.states, tt.candidate)
			if got != tt.want {
				t.Errorf("decideReassert() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
