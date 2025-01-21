package sql

import (
	"context"
	"errors"
	"fmt"
)

type DatabaseOpts struct {
	CharSet   string
	Collation string
}

// CreateDatabase creates a new dolt database with the specified name.
// If the database already exists, it will be skipped.
func (c *Client) CreateDatabase(ctx context.Context, database string, opts DatabaseOpts) error {
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", database)

	if opts.CharSet != "" {
		query += fmt.Sprintf(" CHARACTER SET = '%s'", opts.CharSet)
	}
	if opts.Collation != "" {
		query += fmt.Sprintf(" COLLATE = '%s'", opts.Collation)
	}
	query += ";"

	return c.Exec(ctx, query)
}

// DropDatabase drops the specified dolt database.
// If the database does not exist, it will be skipped.
func (c *Client) DropDatabase(ctx context.Context, database string) error {
	return c.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", database))
}

// CreateBranches creates branches;
// If a branch already exists, it will be skipped.
func (c *Client) CreateBranches(ctx context.Context, branches []string) error {
	for _, branch := range branches {
		if err := c.createBranch(ctx, branch); err != nil {
			if errors.Is(err, ErrBranchExists) {
				continue
			}
		}
	}
	return nil
}

// UseDatabase sets the active database.
func (c *Client) UseDatabase(ctx context.Context, database string) error {
	return c.Exec(ctx, fmt.Sprintf("USE %s;", database))
}
