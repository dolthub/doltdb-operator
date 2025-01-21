package sql

import "fmt"

var (
	ErrInvalidUserIdentifier = fmt.Errorf("invalid user identifier")
	ErrBranchExists          = fmt.Errorf("branch already exists")
)
