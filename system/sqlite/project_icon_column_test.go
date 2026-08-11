package sqlite

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/apito.io/open_driver/system/sqlcommon"
)

func TestEnsureProjectIconColumn(t *testing.T) {
	cfg := &models.Config{}
	cred := &models.DriverCredentials{Engine: "sqlite", Database: ":memory:"}
	drv, err := GetSQLiteSystemDriver(cfg, cred)
	require.NoError(t, err)
	t.Cleanup(func() { _ = drv.Close() })

	ctx := context.Background()
	_, err = drv.ORM.ExecContext(ctx, `
CREATE TABLE projects (
  id TEXT PRIMARY KEY,
  name TEXT,
  roles TEXT
)`)
	require.NoError(t, err)

	require.NoError(t, sqlcommon.EnsureProjectIconColumn(ctx, drv.ORM))

	var name string
	err = drv.ORM.QueryRowContext(ctx,
		`SELECT name FROM pragma_table_info('projects') WHERE name = ?`, "project_icon").Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "project_icon", name)

	require.NoError(t, sqlcommon.EnsureProjectIconColumn(ctx, drv.ORM))
}
