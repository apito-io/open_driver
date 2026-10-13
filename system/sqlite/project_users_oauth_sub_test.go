package sqlite

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/apito-io/open_driver/system/sqlcommon"
)

func TestRunMigrationAddsProjectUsersOAuthSubColumn(t *testing.T) {
	cfg := &models.Config{}
	cred := &models.DriverCredentials{Engine: "sqlite", Database: ":memory:"}
	drv, err := GetSQLiteSystemDriver(cfg, cred)
	require.NoError(t, err)
	t.Cleanup(func() { _ = drv.Close() })

	ctx := context.Background()
	_, err = drv.ORM.ExecContext(ctx, `
CREATE TABLE project_users (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  username TEXT NOT NULL,
  email TEXT,
  phone TEXT,
  secret TEXT,
  role TEXT NOT NULL,
  provider TEXT NOT NULL,
  google_sub TEXT,
  status TEXT NOT NULL,
  created_at TEXT,
  updated_at TEXT
)`)
	require.NoError(t, err)

	require.NoError(t, sqlcommon.EnsureProjectUsersOAuthSubColumn(ctx, drv.ORM))

	var name string
	err = drv.ORM.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('project_users') WHERE name = 'oauth_sub'`).Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "oauth_sub", name)

	require.NoError(t, sqlcommon.EnsureProjectUsersOAuthSubColumn(ctx, drv.ORM))

	row := &models.User{
		ID:        "u1",
		ProjectID: "p1",
		Username:  "alice",
		Email:     "a@example.com",
		Phone:     "01700000000",
		Secret:    "hash",
		Role:      "admin",
		Provider:  models.UserProviderLocal,
		Status:    models.UserStatusActive,
	}
	_, err = drv.CreateUser(ctx, row)
	require.NoError(t, err)
}
