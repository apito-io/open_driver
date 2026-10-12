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
	"github.com/vektah/gqlparser/v2/ast"
)

func TestAggregateDocOfProject_groupsByRelationKeys(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:agg_groupby_test?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
CREATE TABLE customer (id TEXT PRIMARY KEY);
CREATE TABLE returnable_item (id TEXT PRIMARY KEY);
CREATE TABLE ledger (
  id TEXT PRIMARY KEY,
  item_type TEXT,
  collectable_due REAL,
  giveable_due REAL,
  customer_id TEXT REFERENCES customer(id),
  returnable_item_id TEXT REFERENCES returnable_item(id)
);
INSERT INTO customer(id) VALUES ('cust1'), ('cust2');
INSERT INTO returnable_item(id) VALUES ('drum1'), ('drum2');
INSERT INTO ledger(id, item_type, collectable_due, giveable_due, customer_id, returnable_item_id) VALUES
 ('r1', 'returnable', 3, 0, 'cust1', 'drum1'),
 ('r2', 'returnable', 2, 0, 'cust1', 'drum1'),
 ('r3', 'returnable', 5, 0, 'cust1', 'drum2'),
 ('r4', 'returnable', 7, 0, 'cust2', 'drum1'),
 ('r5', 'product',    9, 0, 'cust1', 'drum1');
`)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	model := &models.ModelType{
		Name: "ledger",
		Fields: []*models.FieldInfo{
			{Identifier: "item_type", InputType: _const.StringInput, FieldType: "text"},
			{Identifier: "collectable_due", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "giveable_due", InputType: _const.DoubleInput, FieldType: "number"},
		},
		Connections: []*models.ConnectionType{
			{Model: "customer", Relation: "has_one", Type: "forward"},
			{Model: "returnable_item", Relation: "has_one", Type: "forward"},
		},
	}
	schemaModels := []*models.ModelType{
		model,
		{Name: "customer", Fields: []*models.FieldInfo{{Identifier: "name", InputType: _const.StringInput}}},
		{Name: "returnable_item", Fields: []*models.FieldInfo{{Identifier: "name", InputType: _const.StringInput}}},
	}

	sel := ast.SelectionSet{
		&ast.Field{
			Name: "groupBy",
			SelectionSet: ast.SelectionSet{
				&ast.Field{Name: "group_by_identifier_1"},
				&ast.Field{Name: "group_by_identifier_2"},
				&ast.Field{Name: "collectable_due"},
				&ast.Field{Name: "giveable_due"},
			},
		},
	}

	param := &models.CommonSystemParams{
		Model:               model,
		ProjectSchemaModels: schemaModels,
		IsAggregateQuery:    true,
		QuerySelectionSets:  &sel,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"where": map[string]interface{}{
					"item_type": map[string]interface{}{"eq": "returnable"},
				},
				"groupBy": []interface{}{
					map[string]interface{}{"key": "customer_id", "value": "system_customer_id"},
					map[string]interface{}{"key": "returnable_item_id", "value": "system_returnable_item_id"},
				},
			},
		},
	}

	got, err := drv.AggregateDocOfProject(ctx, param)
	require.NoError(t, err)
	rows, ok := got.([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, rows, 3)

	sums := make(map[string]float64, len(rows))
	for _, row := range rows {
		key := row["group_by_identifier_1"].(string) + "/" + row["group_by_identifier_2"].(string)
		sums[key] = row["collectable_due"].(float64)
	}
	require.InDelta(t, 5, sums["cust1/drum1"], 0.001)
	require.InDelta(t, 5, sums["cust1/drum2"], 0.001)
	require.InDelta(t, 7, sums["cust2/drum1"], 0.001)
}

func TestGroupBySelectExpressions_rejectsUnknownField(t *testing.T) {
	model := &models.ModelType{
		Name:   "ledger",
		Fields: []*models.FieldInfo{{Identifier: "item_type", InputType: _const.StringInput, FieldType: "text"}},
		Connections: []*models.ConnectionType{
			{Model: "customer", Relation: "has_one", Type: "forward"},
		},
	}

	selectExprs, groupCols, err := GroupBySelectExpressions(model, []GroupByKey{
		{Key: "customer_id", Value: "system_customer_id"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"`x`.`customer_id` AS `group_by_identifier_1`"}, selectExprs)
	require.Equal(t, []string{"`x`.`customer_id`"}, groupCols)

	_, _, err = GroupBySelectExpressions(model, []GroupByKey{{Key: "nope", Value: "not_a_field"}})
	require.Error(t, err)
}
