package sqlcommon

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const colPlanSeedOmissions = "plan_seed_omissions"

// EnsureProjectPlanSeedOmissionsColumn adds projects.plan_seed_omissions (JSON string array).
func EnsureProjectPlanSeedOmissionsColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for projects.plan_seed_omissions migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s TEXT`, tableProjects, colPlanSeedOmissions))
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s JSONB`, tableProjects, colPlanSeedOmissions))
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s JSON NULL`, tableProjects, colPlanSeedOmissions))
	default:
		return nil
	}
}
