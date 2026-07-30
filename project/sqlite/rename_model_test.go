package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestRenameModel_SQLiteAlterTableSyntax(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE product (id VARCHAR(36) NOT NULL PRIMARY KEY, name TEXT);
CREATE TABLE vendor (id VARCHAR(36) NOT NULL PRIMARY KEY, product_id VARCHAR(36));
CREATE TABLE product_tag (
  product_id VARCHAR(36) NOT NULL,
  tag_id VARCHAR(36) NOT NULL,
  PRIMARY KEY (product_id, tag_id)
);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	project := &models.Project{
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				{
					Name: "product",
					Fields: []*models.FieldInfo{
						{Identifier: "name", FieldType: "text"},
					},
					Connections: []*models.ConnectionType{
						{Model: "tag", Relation: "has_many", Type: "forward"},
						{Model: "vendor", Relation: "has_one", Type: "backward"},
					},
				},
				{
					Name: "vendor",
					Connections: []*models.ConnectionType{
						{Model: "product", Relation: "has_one", Type: "forward"},
					},
				},
				{
					Name: "tag",
					Connections: []*models.ConnectionType{
						{Model: "product", Relation: "has_many", Type: "backward"},
					},
				},
			},
		},
	}

	require.NoError(t, drv.RenameModel(ctx, project, "product", "category"))

	assertTable(t, db, ctx, "product", false)
	assertTable(t, db, ctx, "category", true)
	assertTable(t, db, ctx, "product_tag", false)
	assertTable(t, db, ctx, "category_tag", true)
	assertColumn(t, db, ctx, "vendor", "product_id", false)
	assertColumn(t, db, ctx, "vendor", "category_id", true)
	assertColumn(t, db, ctx, "category_tag", "category_id", true)
	assertColumn(t, db, ctx, "category_tag", "tag_id", true)

	found := false
	for _, m := range project.Schema.Models {
		if m.Name == "category" {
			found = true
		}
		if m.Name == "product" {
			t.Fatal("schema still has product after rename")
		}
	}
	require.True(t, found, "schema model name should update to category")
}
