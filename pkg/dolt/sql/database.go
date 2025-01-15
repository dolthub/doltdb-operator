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

var (
	ErrBranchExists = fmt.Errorf("branch already exists")
)

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
	for _, pattern := range patterns {
		query := "INSERT INTO dolt_ignore (pattern, ignored) VALUES (?, true) " +
			"ON DUPLICATE KEY UPDATE ignored = VALUES(ignored);"

		if err := c.Exec(ctx, query, pattern); err != nil {
			return fmt.Errorf("error inserting or updating pattern '%s': %w", pattern, err)
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
