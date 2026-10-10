package sqlcommon

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const tableModelTypes = "model_types"

// EnsureModelTypeUniqueKeysColumn adds model_types.unique_keys on databases created
// before composite unique keys existed. CreateTable IF NOT EXISTS does not alter them.
func EnsureModelTypeUniqueKeysColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for model_types unique_keys migration")
	}
	var err error
	switch db.Dialect().Name() {
	case dialect.SQLite:
		err = ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableModelTypes+` ADD COLUMN unique_keys VARCHAR`)
	case dialect.PG:
		_, err = db.ExecContext(ctx,
			`ALTER TABLE `+tableModelTypes+` ADD COLUMN IF NOT EXISTS unique_keys JSONB`)
	case dialect.MySQL:
		err = ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableModelTypes+` ADD COLUMN unique_keys JSON NULL`)
	default:
		return nil
	}
	return err
}
