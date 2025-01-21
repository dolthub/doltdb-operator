package sql

import (
	"context"
	"fmt"
	"strings"
)

type grantOpts struct {
	grantOption bool
}

type GrantOption func(*grantOpts)

func WithGrantOption() GrantOption {
	return func(o *grantOpts) {
		o.grantOption = true
	}
}

// Grant grants specified privileges on a given database and table to an account.
// It constructs a SQL GRANT statement and executes it.
func (c *Client) Grant(
	ctx context.Context,
	privileges []string,
	database string,
	table string,
	accountName string,
	opts ...GrantOption,
) error {
	var grantOpts grantOpts
	for _, setOpt := range opts {
		setOpt(&grantOpts)
	}

	query := fmt.Sprintf("GRANT %s ON %s.%s TO %s ",
		strings.Join(privileges, ","),
		escapeWildcard(database),
		escapeWildcard(table),
		accountName,
	)
	if grantOpts.grantOption {
		query += "WITH GRANT OPTION "
	}
	query += ";"

	return c.Exec(ctx, query)
}

// Revoke revokes the specified privileges from a given account
// on a specific table within a database.
func (c *Client) Revoke(
	ctx context.Context,
	privileges []string,
	database string,
	table string,
	accountName string,
	opts ...GrantOption,
) error {
	var grantOpts grantOpts
	for _, setOpt := range opts {
		setOpt(&grantOpts)
	}

	if grantOpts.grantOption {
		privileges = append(privileges, "GRANT OPTION")
	}
	query := fmt.Sprintf("REVOKE %s ON %s.%s FROM %s",
		strings.Join(privileges, ","),
		escapeWildcard(database),
		escapeWildcard(table),
		accountName,
	)

	return c.Exec(ctx, query)
}

func escapeWildcard(s string) string {
	if s == "*" {
		return s
	}
	return fmt.Sprintf("`%s`", s)
}
