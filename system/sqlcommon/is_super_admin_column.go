package sqlcommon

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const colIsSuperAdmin = "is_super_admin"

// EnsureSystemUserIsSuperAdminColumn adds system_users.is_super_admin for existing
// databases. Bun CreateTable().IfNotExists() does not alter existing tables.
func EnsureSystemUserIsSuperAdminColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for system_users.is_super_admin migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN `+colIsSuperAdmin+` INTEGER DEFAULT 0`)
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN IF NOT EXISTS `+colIsSuperAdmin+` BOOLEAN DEFAULT FALSE`)
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN `+colIsSuperAdmin+` TINYINT(1) DEFAULT 0`)
	default:
		return nil
	}
}
