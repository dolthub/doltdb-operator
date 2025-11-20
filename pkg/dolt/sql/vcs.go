// Copyright (c) 2025 Electronic Arts Inc. All rights reserved.

package sql

import (
	"context"
	"database/sql"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GetBranches returns a list of branches;
func (c *Client) GetBranches(ctx context.Context) ([]string, error) {
	query := "SELECT name FROM dolt_branches"
	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error getting branches: %w", err)
	}

	var name string
	var branches []string
	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("error scanning branch name: %w", err)
		}
		branches = append(branches, name)
	}

	return branches, nil
}

func (c *Client) createBranch(ctx context.Context, branch string) error {
	query := "SELECT COUNT(*) FROM dolt_branches WHERE name = ?;"
	row := c.QueryRow(ctx, query, branch)
	if row.Err() != nil {
		return fmt.Errorf("error checking if branch '%s' exists: %w", branch, row.Err())
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrBranchExists
	}

	return c.Exec(ctx, "CALL DOLT_BRANCH('-c', 'main', ?)", branch)
}

type IgnoreOpts struct {
	Patterns []string
}

// CreateDoltIgnore adds patterns to the dolt_ignore table.
// It excludes tables from being tracked by Dolt.
// If a pattern already exists, it will be updated to be ignored.
// Ref: https://www.dolthub.com/blog/2023-05-03-using-dolt_ignore-to-prevent-accidents/
func (c *Client) CreateDoltIgnore(ctx context.Context, patterns []string) error {
	log := log.FromContext(ctx)
	tx, err := c.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("error beginning transaction to create dolt ignore: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Error(rollbackErr, "transaction rollback error")
			}
			panic(p)
		} else if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Error(rollbackErr, "transaction rollback error")
			}
		} else {
			err = tx.Commit()
		}
	}()

	for _, pattern := range patterns {
		var count int
		query := "SELECT COUNT(*) FROM dolt_ignore WHERE pattern = ?"
		row := tx.QueryRowContext(ctx, query, pattern)
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("error checking if pattern '%s' exists: %w", pattern, err)
		}

		if count == 0 {
			query = "INSERT INTO dolt_ignore (pattern, ignored) VALUES (?, true)"
			if _, err := tx.ExecContext(ctx, query, pattern); err != nil {
				return fmt.Errorf("error inserting pattern '%s': %w", pattern, err)
			}
		}
	}

	return nil
}

type DoltIgnore struct {
	Pattern string
	Ignored bool
}

// GetDoltIgnore returns a list of patterns in the dolt_ignore table.
func (c *Client) GetDoltIgnore(ctx context.Context) ([]DoltIgnore, error) {
	query := "SELECT pattern, ignored FROM dolt_ignore"
	rows, err := c.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error getting dolt ignore entries: %w", err)
	}

	var doltIgnores []DoltIgnore
	for rows.Next() {
		var row DoltIgnore
		if err := rows.Scan(&row.Pattern, &row.Ignored); err != nil {
			return nil, fmt.Errorf("error scanning dolt ignore entry: %w", err)
		}
		doltIgnores = append(doltIgnores, row)
	}

	return doltIgnores, nil
}
