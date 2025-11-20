// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package statefulset

import "errors"

var (
	ErrSkipReconciliationPhase = errors.New("skipping reconciliation phase")
)
