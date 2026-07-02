package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestPreflightSQLiteRelationParentTables_usersTable(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	tenant_id TEXT,
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
);
CREATE TABLE schedule (id TEXT PRIMARY KEY, title TEXT);
`).Exec(ctx)
	require.NoError(t, err)

	from := &models.ConnectionType{Model: "schedule", Relation: "has_many", Type: "forward"}
	to := &models.ConnectionType{Model: "users", Relation: "has_one", Type: "backward"}
	require.NoError(t, PreflightSQLiteRelationParentTablesForAddRelation(ctx, db, _const.SQLiteDriver, from, to))
}

func TestAddRelationFields_scheduleHasOneUsers(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL,
	role TEXT NOT NULL,
	provider TEXT NOT NULL,
	status TEXT NOT NULL
);
CREATE TABLE schedule (id TEXT PRIMARY KEY, title TEXT);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	from := &models.ConnectionType{Model: "users", Relation: "has_one"}
	to := &models.ConnectionType{Model: "schedule", Relation: "has_many"}
	require.NoError(t, drv.AddRelationFields(ctx, from, to))
	assertColumn(t, db, ctx, "schedule", "users_id", true)
}

func TestRootResolverQueryBuilder_bookHasOneUsersConnection(t *testing.T) {
	usersModel := models.DefaultProjectAuthUserModel()
	bookModel := &models.ModelType{
		Name: "book",
		Connections: []*models.ConnectionType{
			{Model: "users", Relation: "has_one", Type: "forward"},
		},
	}
	schema := []*models.ModelType{bookModel, usersModel}
	param := &models.CommonSystemParams{
		Model:               usersModel,
		ProjectSchemaModels: schema,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"local": "en",
				"connection": map[string]interface{}{
					"connection_type": "forward",
					"relation_type":   "has_one",
					"model":           "users",
					"to_model":        "book",
					"_id":             "01BOOK",
				},
			},
		},
	}
	q, args, err := RootResolverQueryBuilder(&models.Config{}, param, false)
	require.NoError(t, err)
	require.Contains(t, q, "EXISTS (SELECT 1 FROM `book` AS anch WHERE anch.id = ? AND anch.`users_id` = `x`.`id`)")
	require.Equal(t, []interface{}{"01BOOK"}, args)
	require.NotContains(t, q, "left join meta")
	require.Contains(t, q, "ORDER BY x.created_at DESC")
	require.NotContains(t, q, "y.created_at")
}

func TestRootResolverQueryBuilder_bookHasOneUsersIntersect(t *testing.T) {
	usersModel := models.DefaultProjectAuthUserModel()
	bookModel := &models.ModelType{
		Name: "book",
		Connections: []*models.ConnectionType{
			{Model: "users", Relation: "has_one", Type: "forward"},
		},
	}
	schema := []*models.ModelType{bookModel, usersModel}
	param := &models.CommonSystemParams{
		Model:               usersModel,
		ProjectSchemaModels: schema,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"local":     "en",
				"intersect": true,
				"connection": map[string]interface{}{
					"connection_type": "forward",
					"relation_type":   "has_one",
					"model":           "users",
					"to_model":        "book",
					"_id":             "01KWA7B399MAC3HYY9BWXW6D6Q",
				},
			},
		},
	}
	q, args, err := RootResolverQueryBuilder(&models.Config{}, param, false)
	require.NoError(t, err)
	require.Contains(t, q, "NOT EXISTS (SELECT 1 FROM `book` AS anch WHERE anch.id = ? AND anch.`users_id` = `x`.`id`)")
	require.Equal(t, []interface{}{"01KWA7B399MAC3HYY9BWXW6D6Q"}, args)
}
