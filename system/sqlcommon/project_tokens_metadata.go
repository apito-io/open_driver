package sqlcommon

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const tableProjectTokens = "project_tokens"

// EnsureProjectTokensMetadataColumns adds token_id / token_prefix /
// token_fingerprint / created_at on existing project_tokens tables.
// Bun CreateTable().IfNotExists() does not alter existing tables, so SELECT
// after ProjectToken struct growth fails with "no such column: …token_id".
func EnsureProjectTokensMetadataColumns(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for project_tokens metadata migration")
	}
	cols := []struct {
		name string
		ddl  string
	}{
		{name: "token_id", ddl: "VARCHAR(64)"},
		{name: "token_prefix", ddl: "VARCHAR(32)"},
		{name: "token_fingerprint", ddl: "VARCHAR(128)"},
		{name: "created_at", ddl: "VARCHAR(64)"},
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		for _, c := range cols {
			if err := ExecAlterIgnoreDuplicate(ctx, db,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableProjectTokens, c.name, c.ddl)); err != nil {
				return err
			}
		}
	case dialect.PG:
		for _, c := range cols {
			if _, err := db.ExecContext(ctx,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s`, tableProjectTokens, c.name, c.ddl)); err != nil {
				return err
			}
		}
	case dialect.MySQL:
		for _, c := range cols {
			if err := ExecAlterIgnoreDuplicate(ctx, db,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s NULL`, tableProjectTokens, c.name, c.ddl)); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	// Helpful for revoke-by-token_id lookups; ignore failures on engines that
	// already have the index or reject IF NOT EXISTS variants.
	_, _ = db.ExecContext(ctx,
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_project_tokens_token_id ON %s (token_id)`, tableProjectTokens))
	return nil
}
