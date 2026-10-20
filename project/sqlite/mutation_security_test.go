package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestUpdateDocumentOfProject_OwnMismatchLeavesDocumentAndMetadataUntouched(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:mutation_security?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	sqldb.SetMaxOpenConns(1)
	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE post (id TEXT PRIMARY KEY, title TEXT NOT NULL, owner_id TEXT NOT NULL);
		CREATE TABLE meta (
			id TEXT PRIMARY KEY,
			doc_id TEXT NOT NULL,
			created_by TEXT NOT NULL,
			updated_by TEXT NOT NULL,
			updated_at DATETIME NOT NULL,
			status TEXT NOT NULL
		);
		INSERT INTO post (id, title, owner_id) VALUES ('doc-a', 'original', 'owner-a');
		INSERT INTO meta (id, doc_id, created_by, updated_by, updated_at, status)
		VALUES ('meta-a', 'doc-a', 'owner-a', 'owner-a', '2026-01-01T00:00:00Z', 'published');
	`)
	require.NoError(t, err)

	driver := testDriver(db, &models.DriverCredentials{Engine: "sqlite"})
	param := &models.CommonSystemParams{
		UserID: "owner-b",
		Role: &models.Role{
			IsProjectUser: true,
			APIPermissions: map[string]*models.APIPermission{
				"post": {Update: "own"},
			},
		},
		Model: &models.ModelType{
			Name:   "post",
			Fields: []*models.FieldInfo{{Identifier: "owner_id"}},
		},
	}
	doc := &types.DefaultDocumentStructure{
		ID:   "doc-a",
		Type: "post",
		Data: map[string]interface{}{"title": "attacker-write"},
	}

	require.NoError(t, driver.UpdateDocumentOfProject(ctx, param, doc, false))

	var title, owner, updatedBy string
	require.NoError(t, db.NewRaw("SELECT title, owner_id FROM post WHERE id = ?", "doc-a").
		Scan(ctx, &title, &owner))
	require.NoError(t, db.NewRaw("SELECT updated_by FROM meta WHERE doc_id = ?", "doc-a").
		Scan(ctx, &updatedBy))
	require.Equal(t, "original", title)
	require.Equal(t, "owner-a", owner)
	require.Equal(t, "owner-a", updatedBy)
}
