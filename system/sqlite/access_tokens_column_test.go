package sqlite

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/apito-io/open_driver/system/sqlcommon"
)

func TestRunMigrationAddsAccessTokensColumn(t *testing.T) {
	cfg := &models.Config{}
	cred := &models.DriverCredentials{Engine: "sqlite", Database: ":memory:"}
	drv, err := GetSQLiteSystemDriver(cfg, cred)
	require.NoError(t, err)
	t.Cleanup(func() { _ = drv.Close() })

	ctx := context.Background()
	// Simulate a pre-apt_ schema: create system_users without access_tokens.
	_, err = drv.ORM.ExecContext(ctx, `
CREATE TABLE system_users (
  id TEXT PRIMARY KEY,
  email TEXT,
  sync_tokens TEXT
)`)
	require.NoError(t, err)

	require.NoError(t, sqlcommon.EnsureSystemUserAccessTokensColumn(ctx, drv.ORM))

	var name string
	err = drv.ORM.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('system_users') WHERE name = 'access_tokens'`).Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "access_tokens", name)

	// Idempotent on second run.
	require.NoError(t, sqlcommon.EnsureSystemUserAccessTokensColumn(ctx, drv.ORM))
}
