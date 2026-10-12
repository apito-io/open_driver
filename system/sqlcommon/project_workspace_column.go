package sqlcommon

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

// EnsureProjectTempBannedColumn adds projects.temp_banned for grace/read-only freeze.
func EnsureProjectTempBannedColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for projects.temp_banned migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN temp_banned INTEGER NOT NULL DEFAULT 0`, tableProjects))
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS temp_banned BOOLEAN NOT NULL DEFAULT FALSE`, tableProjects))
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN temp_banned TINYINT(1) NOT NULL DEFAULT 0`, tableProjects))
	default:
		return nil
	}
}
