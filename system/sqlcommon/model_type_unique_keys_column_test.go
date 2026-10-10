package sqlcommon

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestEnsureModelTypeUniqueKeysColumn(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:model-unique-keys?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
CREATE TABLE model_types (
  project_id VARCHAR NOT NULL,
  name TEXT NOT NULL,
  description VARCHAR,
  fields VARCHAR,
  connections VARCHAR,
  hook_ids VARCHAR,
  locals VARCHAR,
  repeated_groups VARCHAR,
  system_generated BOOLEAN,
  single_page BOOLEAN,
  single_page_uuid VARCHAR,
  has_connections BOOLEAN,
  enable_revision BOOLEAN,
  revision_filter VARCHAR,
  ext VARCHAR,
  PRIMARY KEY (project_id, name)
)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO model_types (project_id, name) VALUES ('p1', 'stock')`)
	require.NoError(t, err)

	require.NoError(t, EnsureModelTypeUniqueKeysColumn(ctx, db))
	require.NoError(t, EnsureModelTypeUniqueKeysColumn(ctx, db))

	var row models.ModelType
	require.NoError(t, db.NewSelect().Model(&row).Where("project_id = ?", "p1").Scan(ctx))
	require.Empty(t, row.UniqueKeys)
}
