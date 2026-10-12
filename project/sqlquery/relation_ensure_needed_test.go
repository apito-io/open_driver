package sqlquery

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func TestRelationEnsureNeeded(t *testing.T) {
	stock := &models.ModelType{Name: "stock"}
	if RelationEnsureNeeded(&models.CommonSystemParams{ProjectSchemaModels: []*models.ModelType{stock}}) {
		t.Fatal("plain list with no unique keys must not ensure")
	}
	stock.UniqueKeys = []*models.UniqueKey{{Name: "variant_warehouse", Columns: []string{"variant_id", "warehouse_id"}}}
	if !RelationEnsureNeeded(&models.CommonSystemParams{ProjectSchemaModels: []*models.ModelType{stock}}) {
		t.Fatal("published unique key must ensure on a plain list")
	}
	plain := &models.ModelType{Name: "note"}
	withConn := &models.CommonSystemParams{
		ProjectSchemaModels: []*models.ModelType{plain},
		ResolveParams: &graphql.ResolveParams{Args: map[string]interface{}{
			"connection": map[string]interface{}{"variant": map[string]interface{}{}},
		}},
	}
	if !RelationEnsureNeeded(withConn) {
		t.Fatal("relation selection must still ensure")
	}
}
