package sqlcommon

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const (
	colAccessTokens = "access_tokens"
	tableSystemUsers = "system_users"
)

// EnsureSystemUserAccessTokensColumn adds system_users.access_tokens for existing
// databases. Bun CreateTable().IfNotExists() does not alter existing tables, so
// apt_ token storage would panic on SELECT until this runs.
func EnsureSystemUserAccessTokensColumn(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for system_users.access_tokens migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		return ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN `+colAccessTokens+` TEXT`)
	case dialect.PG:
		_, err := db.ExecContext(ctx,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN IF NOT EXISTS `+colAccessTokens+` JSONB`)
		return err
	case dialect.MySQL:
		return ExecAlterIgnoreDuplicate(ctx, db,
			`ALTER TABLE `+tableSystemUsers+` ADD COLUMN `+colAccessTokens+` JSON NULL`)
	default:
		return nil
	}
}
