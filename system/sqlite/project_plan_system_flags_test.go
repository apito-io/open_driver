package sqlite

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"gitlab.com/apito.io/open_driver/system/sqlcommon"
)

func TestNormalizeAllProjectPlanSystemFlags(t *testing.T) {
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
  plans TEXT,
  plan_seed_omissions TEXT,
  updated_at TEXT
)`)
	require.NoError(t, err)

	plans, err := json.Marshal(map[string]*models.Plan{
		"free":  {ID: "free", Name: "Free", SystemGenerated: true},
		"paid":  {ID: "paid", Name: "Pro", SystemGenerated: true},
		"ultra": {ID: "ultra", Name: "Ultra", SystemGenerated: true},
	})
	require.NoError(t, err)
	_, err = drv.ORM.ExecContext(ctx,
		`INSERT INTO projects (id, name, plans) VALUES (?, ?, ?)`,
		"proj_1", "Demo", string(plans))
	require.NoError(t, err)

	require.NoError(t, sqlcommon.NormalizeAllProjectPlanSystemFlags(ctx, drv.ORM))

	var raw string
	require.NoError(t, drv.ORM.QueryRowContext(ctx, `SELECT plans FROM projects WHERE id = ?`, "proj_1").Scan(&raw))
	var out map[string]*models.Plan
	require.NoError(t, json.Unmarshal([]byte(raw), &out))
	require.True(t, out["free"].SystemGenerated)
	require.False(t, out["paid"].SystemGenerated)
	require.False(t, out["ultra"].SystemGenerated)

	require.NoError(t, sqlcommon.NormalizeAllProjectPlanSystemFlags(ctx, drv.ORM))
}
