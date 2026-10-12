package sqlcommon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const colOAuthSub = "oauth_sub"

// EnsureProjectUsersOAuthSubColumn adds project_users.oauth_sub for existing
// databases. Bun CreateTable().IfNotExists() does not alter existing tables, so
// multi-provider OAuth (and Pro createUser uniqueness SELECTs) fail with
// "no such column: pu.oauth_sub" until this runs.
func EnsureProjectUsersOAuthSubColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for project_users.oauth_sub migration")
	}
	table := models.ProjectUsersTableName
	switch db.Dialect().Name() {
	case dialect.SQLite:
		if err := ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s VARCHAR(255)`, table, colOAuthSub)); err != nil {
			return err
		}
	case dialect.PG:
		if _, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s VARCHAR(255)`, table, colOAuthSub)); err != nil {
			return err
		}
	case dialect.MySQL:
		if err := ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s VARCHAR(255)`, table, colOAuthSub)); err != nil {
			return err
		}
	default:
		return nil
	}
	return ensureProjectUsersOAuthSubIndexes(ctx, db, table)
}

func ensureProjectUsersOAuthSubIndexes(ctx context.Context, db *bun.DB, table string) error {
	stmts := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_project_users_oauth_sub ON %s (project_id, provider, oauth_sub)`, table),
		fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_users_oauth_sub_unique ON %s (project_id, provider, oauth_sub) WHERE TRIM(COALESCE(oauth_sub, '')) != ''`, table),
	}
	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			// MySQL lacks partial unique indexes; duplicate data can block unique indexes.
			if db.Dialect().Name() == dialect.MySQL || strings.Contains(strings.ToLower(q), "unique index") {
				continue
			}
			return err
		}
	}
	return nil
}
