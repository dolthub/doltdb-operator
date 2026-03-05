// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// sanitizeIdentifier validates and sanitizes a SQL identifier for safe use
// in backtick-quoted contexts. Strips backticks (the only escape char in
// backtick-quoted identifiers) and rejects names that don't match the
// allowlist. CRD-level CEL validation is the primary gate; this is
// defense-in-depth.
func sanitizeIdentifier(name string) (string, error) {
	s := strings.ReplaceAll(name, "`", "")
	if s == "" || !validIdentifier.MatchString(s) {
		return "", fmt.Errorf("invalid identifier: %q", name)
	}
	return s, nil
}

type DatabaseOpts struct {
	CharSet   string
	Collation string
}

// CreateDatabase creates a new dolt database with the specified name.
// If the database already exists, it will be skipped.
func (c *Client) CreateDatabase(ctx context.Context, database string, opts DatabaseOpts) error {
	safe, err := sanitizeIdentifier(database)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", safe)

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
	safe, err := sanitizeIdentifier(database)
	if err != nil {
		return err
	}
	return c.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", safe))
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
	safe, err := sanitizeIdentifier(database)
	if err != nil {
		return err
	}
	return c.Exec(ctx, fmt.Sprintf("USE `%s`;", safe))
}
