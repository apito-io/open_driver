package sqlcommon

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const colProjectIcon = "project_icon"

// EnsureProjectIconColumn adds projects.project_icon (branding URL for receipts).
// Bun CreateTable().IfNotExists() does not alter existing tables, so SELECT
// after Project.ProjectIcon growth fails with "no such column: project.project_icon".
func EnsureProjectIconColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for projects.project_icon migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s TEXT`, tableProjects, colProjectIcon))
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s VARCHAR`, tableProjects, colProjectIcon))
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s VARCHAR(2048) NULL`, tableProjects, colProjectIcon))
	default:
		return nil
	}
}
