package sqlite

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/apito.io/open_driver/system/sqlcommon"
)

func TestEnsureProjectTokensMetadataColumns(t *testing.T) {
	cfg := &models.Config{}
	cred := &models.DriverCredentials{Engine: "sqlite", Database: ":memory:"}
	drv, err := GetSQLiteSystemDriver(cfg, cred)
	require.NoError(t, err)
	t.Cleanup(func() { _ = drv.Close() })

	ctx := context.Background()
	_, err = drv.ORM.ExecContext(ctx, `
CREATE TABLE project_tokens (
  project_id TEXT,
  name TEXT,
  token TEXT,
  role TEXT,
  expire TEXT
)`)
	require.NoError(t, err)

	require.NoError(t, sqlcommon.EnsureProjectTokensMetadataColumns(ctx, drv.ORM))

	for _, col := range []string{"token_id", "token_prefix", "token_fingerprint", "created_at"} {
		var name string
		err = drv.ORM.QueryRowContext(ctx,
			`SELECT name FROM pragma_table_info('project_tokens') WHERE name = ?`, col).Scan(&name)
		require.NoError(t, err, col)
		require.Equal(t, col, name)
	}

	// Idempotent
	require.NoError(t, sqlcommon.EnsureProjectTokensMetadataColumns(ctx, drv.ORM))
}
