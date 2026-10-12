package sqlcommon

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const (
	tableProjects = "projects"
	colPlans      = "plans"
)

// EnsureProjectPlansColumn adds projects.plans (JSON map of Plan ceilings).
// Bun CreateTable().IfNotExists() does not alter existing tables, so SELECT
// after Project.Plans growth fails with "no such column: project.plans".
func EnsureProjectPlansColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for projects.plans migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s TEXT`, tableProjects, colPlans))
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s JSONB`, tableProjects, colPlans))
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s JSON NULL`, tableProjects, colPlans))
	default:
		return nil
	}
}
