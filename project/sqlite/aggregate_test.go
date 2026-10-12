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

func TestAggregateSelectExpressions_numericOnly(t *testing.T) {
	model := &models.ModelType{
		Name: "ledger",
		Fields: []*models.FieldInfo{
			{Identifier: "payment_amount", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "item_type", InputType: _const.StringInput, FieldType: "text"},
			{Identifier: "total_quantity", InputType: _const.IntInput, FieldType: "number"},
		},
	}
	exprs, err := AggregateSelectExpressions(model, []string{"payment_amount", "total_quantity"})
	require.NoError(t, err)
	require.Equal(t, []string{
		"COALESCE(SUM(`x`.`payment_amount`), 0) AS `payment_amount`",
		"COALESCE(SUM(`x`.`total_quantity`), 0) AS `total_quantity`",
	}, exprs)

	_, err = AggregateSelectExpressions(model, []string{"item_type"})
	require.Error(t, err)
}

func TestAggregateDocOfProject_sumsWithWhereAndRelation(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:agg_test?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx := context.Background()

	_, err = db.ExecContext(ctx, `
CREATE TABLE customer (id TEXT PRIMARY KEY);
CREATE TABLE ledger_account (id TEXT PRIMARY KEY);
CREATE TABLE ledger (
  id TEXT PRIMARY KEY,
  item_type TEXT,
  payment_amount REAL,
  collectable_due REAL,
  giveable_due REAL,
  instant_profit REAL,
  total_quantity INTEGER,
  customer_id TEXT REFERENCES customer(id),
  ledger_account_id TEXT REFERENCES ledger_account(id)
);
CREATE INDEX idx_ledger_customer_id ON ledger(customer_id);
CREATE INDEX idx_ledger_ledger_account_id ON ledger(ledger_account_id);
INSERT INTO customer(id) VALUES ('cust1'), ('cust2');
INSERT INTO ledger_account(id) VALUES ('acc1');
INSERT INTO ledger(id, item_type, payment_amount, collectable_due, giveable_due, instant_profit, total_quantity, customer_id, ledger_account_id) VALUES
 ('r1', 'product', 500, 200, 0, 0, -5, 'cust1', 'acc1'),
 ('r2', 'product', 2200, 0, 0, 0, -10, 'cust1', 'acc1'),
 ('r3', 'product', 100, 0, 0, 0, 1, 'cust2', 'acc1'),
 ('r4', 'service', 999, 0, 0, 0, 0, 'cust1', 'acc1');
`)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	model := &models.ModelType{
		Name: "ledger",
		Fields: []*models.FieldInfo{
			{Identifier: "item_type", InputType: _const.StringInput, FieldType: "text"},
			{Identifier: "payment_amount", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "collectable_due", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "giveable_due", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "instant_profit", InputType: _const.DoubleInput, FieldType: "number"},
			{Identifier: "total_quantity", InputType: _const.IntInput, FieldType: "number"},
		},
		Connections: []*models.ConnectionType{
			{Model: "customer", Relation: "has_one", Type: "forward"},
			{Model: "ledger_account", Relation: "has_one", Type: "forward"},
		},
	}
	schemaModels := []*models.ModelType{
		model,
		{Name: "customer", Fields: []*models.FieldInfo{{Identifier: "name", InputType: _const.StringInput}}},
		{Name: "ledger_account", Fields: []*models.FieldInfo{{Identifier: "name", InputType: _const.StringInput}}},
	}

	sel := ast.SelectionSet{
		&ast.Field{
			Name: "aggregate",
			SelectionSet: ast.SelectionSet{
				&ast.Field{Name: "payment_amount"},
				&ast.Field{Name: "collectable_due"},
				&ast.Field{Name: "giveable_due"},
				&ast.Field{Name: "instant_profit"},
				&ast.Field{Name: "total_quantity"},
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
					"item_type": map[string]interface{}{"eq": "product"},
				},
				"relation": map[string]interface{}{
					"ledgerAccount": map[string]interface{}{
						"_id": map[string]interface{}{"eq": "acc1"},
					},
					"customer": map[string]interface{}{
						"_id": map[string]interface{}{"eq": "cust1"},
					},
				},
			},
		},
	}

	got, err := drv.AggregateDocOfProject(ctx, param)
	require.NoError(t, err)
	m, ok := got.(map[string]interface{})
	require.True(t, ok)
	require.InDelta(t, 2700, m["payment_amount"].(float64), 0.001)
	require.InDelta(t, 200, m["collectable_due"].(float64), 0.001)
	require.InDelta(t, 0, m["giveable_due"].(float64), 0.001)
	require.InDelta(t, 0, m["instant_profit"].(float64), 0.001)
	require.InDelta(t, -15, m["total_quantity"].(float64), 0.001)
}
