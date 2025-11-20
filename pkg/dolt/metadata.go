// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package dolt

const (
	RoleLabel             = "k8s.dolthub.com/cluster-role"
	VolumeRoleLabel       = "pvc.k8s.dolthub.com/role"
	WatchLabel            = "k8s.dolthub.com/watch"
	Annotation            = "k8s.dolthub.com/doltdb"
	ReplicationAnnotation = "k8s.dolthub.com/replication"

	UserFinalizerName     = "user.k8s.dolthub.com/finalizer"
	DatabaseFinalizerName = "database.k8s.dolthub.com/finalizer"
	GrantFinalizerName    = "grant.k8s.dolthub.com/finalizer"
)

type Role string

const (
	PrimaryRoleValue Role = "primary"
	StandbyRoleValue Role = "standby"
)

func (d Role) String() string {
	return string(d)
}
