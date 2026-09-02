package sqlcommon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const tableUserProjects = "user_projects"

// EnsureUserProjectInviteColumns adds invite lifecycle columns on existing DBs.
func EnsureUserProjectInviteColumns(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for user_projects invite migration")
	}
	cols := []struct {
		name string
		sqlite, pg, mysql string
	}{
		{"invite_status", "TEXT", "VARCHAR", "VARCHAR(32) NULL"},
		{"invited_at", "TEXT", "VARCHAR", "VARCHAR(64) NULL"},
		{"accepted_at", "TEXT", "VARCHAR", "VARCHAR(64) NULL"},
		{"invite_expires_at", "TEXT", "VARCHAR", "VARCHAR(64) NULL"},
		{"invite_token", "TEXT", "VARCHAR", "VARCHAR(64) NULL"},
	}
	for _, col := range cols {
		var err error
		switch db.Dialect().Name() {
		case dialect.SQLite:
			err = ExecAlterIgnoreDuplicate(ctx, db,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableUserProjects, col.name, col.sqlite))
		case dialect.PG:
			_, err = db.ExecContext(ctx,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s`, tableUserProjects, col.name, col.pg))
		case dialect.MySQL:
			err = ExecAlterIgnoreDuplicate(ctx, db,
				fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableUserProjects, col.name, col.mysql))
		default:
			return nil
		}
		if err != nil {
			return err
		}
	}
	switch db.Dialect().Name() {
	case dialect.SQLite, dialect.PG:
		_, err := db.ExecContext(ctx,
			`CREATE INDEX IF NOT EXISTS user_projects_invite_token ON `+tableUserProjects+` (invite_token)`)
		return err
	case dialect.MySQL:
		_, err := db.ExecContext(ctx,
			`CREATE INDEX user_projects_invite_token ON `+tableUserProjects+` (invite_token)`)
		if err == nil {
			return nil
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "already exists") {
			return nil
		}
		return err
	default:
		return nil
	}
}
