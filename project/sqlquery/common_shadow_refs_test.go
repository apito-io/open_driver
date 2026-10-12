package sqlquery

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestCommonModelShadowRefSources(t *testing.T) {
	common := &models.ModelType{Name: "catalog_item", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	tenant := &models.ModelType{
		Name: "tenant_record",
		Connections: []*models.ConnectionType{
			{Model: "catalog_item", Relation: "has_one", Type: "forward"},
		},
	}
	got := CommonModelShadowRefSources([]*models.ModelType{common, tenant}, common)
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	if got[0].Table != "tenant_record" || got[0].Column != "catalog_item_id" {
		t.Fatalf("fk source: %#v", got[0])
	}
}

func TestCommonModelShadowRefSourcesHasManyPivot(t *testing.T) {
	common := &models.ModelType{Name: "catalog_item", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	tenant := &models.ModelType{
		Name: "tenant_record",
		Connections: []*models.ConnectionType{
			{Model: "catalog_item", Relation: "has_many", Type: "forward"},
		},
	}
	got := CommonModelShadowRefSources([]*models.ModelType{common, tenant}, common)
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	if got[0].Table != "catalog_item_tenant_record" || got[0].Column != "catalog_item_id" {
		t.Fatalf("pivot source: %#v", got[0])
	}
}
