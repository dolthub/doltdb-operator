package dolt

const RoleLabel = "dolthub.com/cluster-role"
const PrimaryRoleValue = "primary"
const StandbyRoleValue = "standby"

type Role int

const (
	RoleUnknown Role = 0
	RolePrimary Role = 1
	RoleStandby Role = 2
)
