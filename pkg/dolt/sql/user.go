package sql

import (
	"context"
	"fmt"
)

type CreateUserOpts struct {
	IdentifiedByPassword string
	IdentifiedBy         string
}

func WithIdentifiedByPassword(password string) CreateUserOpt {
	return func(cuo *CreateUserOpts) {
		cuo.IdentifiedByPassword = password
	}
}

func WithIdentifiedBy(password string) CreateUserOpt {
	return func(cuo *CreateUserOpts) {
		cuo.IdentifiedBy = password
	}
}

type CreateUserOpt func(*CreateUserOpts)

// CreateUser creates a new user in the database with the specified account name and options.
// The function constructs a SQL query to create the user and executes it.
// If the user already exists, it will be a noop.
// If user identifier is not provided, returns ErrInvalidUserIdentifier
func (c *Client) CreateUser(ctx context.Context, accountName string, createUserOpts ...CreateUserOpt) error {
	opts := CreateUserOpts{}
	for _, setOpt := range createUserOpts {
		setOpt(&opts)
	}

	query := fmt.Sprintf("CREATE USER IF NOT EXISTS %s ", accountName)

	if opts.IdentifiedByPassword != "" {
		query += fmt.Sprintf("IDENTIFIED BY PASSWORD '%s' ", opts.IdentifiedByPassword)
	} else if opts.IdentifiedBy != "" {
		query += fmt.Sprintf("IDENTIFIED BY '%s' ", opts.IdentifiedBy)
	} else {
		return ErrInvalidUserIdentifier
	}

	query += ";"

	return c.Exec(ctx, query)
}

// DropUser drops a user from the database if the user exists.
func (c *Client) DropUser(ctx context.Context, accountName string) error {
	query := fmt.Sprintf("DROP USER IF EXISTS %s;", accountName)

	return c.Exec(ctx, query)
}

// AlterUser modifies an existing user account in the database with the specified options.
// It supports altering the user's password or identification method.
func (c *Client) AlterUser(ctx context.Context, accountName string, createUserOpts ...CreateUserOpt) error {
	opts := CreateUserOpts{}
	for _, setOpt := range createUserOpts {
		setOpt(&opts)
	}

	query := fmt.Sprintf("ALTER USER %s ", accountName)

	if opts.IdentifiedByPassword != "" {
		query += fmt.Sprintf("IDENTIFIED BY PASSWORD '%s' ", opts.IdentifiedByPassword)
	} else {
		query += fmt.Sprintf("IDENTIFIED BY '%s' ", opts.IdentifiedBy)
	}

	query += ";"

	return c.Exec(ctx, query)
}

// UserExists checks if a user with the specified username and host exists in the MySQL database.
// It returns true if the user exists, otherwise false. If an error occurs during the query, it returns false and the error.
// Ref: https://docs.dolthub.com/sql-reference/server/access-management#statements
func (c *Client) UserExists(ctx context.Context, username, host string) (bool, error) {
	row := c.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mysql.user WHERE user=? AND host=?", username, host)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
