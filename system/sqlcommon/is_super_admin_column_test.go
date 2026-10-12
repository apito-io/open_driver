package sqlcommon

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestEnsureSystemUserIsSuperAdminColumn(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:is-super-admin-col?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
CREATE TABLE system_users (
  id TEXT PRIMARY KEY,
  email TEXT
)`)
	require.NoError(t, err)
	require.NoError(t, EnsureSystemUserIsSuperAdminColumn(ctx, db))

	var name string
	err = db.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('system_users') WHERE name = 'is_super_admin'`).Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "is_super_admin", name)
	require.NoError(t, EnsureSystemUserIsSuperAdminColumn(ctx, db))
}
