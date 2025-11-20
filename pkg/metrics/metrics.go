// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	defaultLabels = []string{"doltdb", "crdnamespace"}

	DoltDBCurrentPrimaryIndex = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "doltdb_current_primary_index",
			Help: "Primary index of the doltdb",
		},
		defaultLabels,
	)
	DoltDBReplicationSwitchOvers = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "doltdb_replication_switch_overs",
			Help: "Number of times there was a replication switch over",
		},
		defaultLabels,
	)
)

func init() {
	metrics.Registry.MustRegister(
		DoltDBCurrentPrimaryIndex,
		DoltDBReplicationSwitchOvers,
	)
}
