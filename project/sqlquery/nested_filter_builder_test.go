package sqlquery

import (
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func customerFixtureModel() *models.ModelType {
	return &models.ModelType{
		Name: "customer",
		Fields: []*models.FieldInfo{
			{Identifier: "status", FieldType: _const.TextField, InputType: _const.StringInput},
			{Identifier: "address", FieldType: _const.ObjectField, SubFieldInfo: []*models.FieldInfo{
				{Identifier: "name", FieldType: _const.TextField, InputType: _const.StringInput, EnableIndexing: true},
				{Identifier: "geo", FieldType: _const.ObjectField, SubFieldInfo: []*models.FieldInfo{
					{Identifier: "lat", FieldType: _const.NumberField, InputType: "double"},
				}},
			}},
			{Identifier: "addresses", FieldType: _const.RepeatedField, SubFieldInfo: []*models.FieldInfo{
				{Identifier: "name", FieldType: _const.TextField, InputType: _const.StringInput, EnableIndexing: true},
			}},
		},
	}
}

func TestFilterBuilder_objectSubfield_eq(t *testing.T) {
	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"address": map[string]interface{}{
			"name": map[string]interface{}{"eq": "NYC"},
		},
	}, customerFixtureModel(), &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 predicate, got %v", preds)
	}
	if !strings.Contains(preds[0], "json_extract") && !strings.Contains(preds[0], "address__name") {
		t.Fatalf("expected json_extract or generated column, got %q", preds[0])
	}
}

func TestFilterBuilder_objectNested_eq(t *testing.T) {
	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"address": map[string]interface{}{
			"geo": map[string]interface{}{
				"lat": map[string]interface{}{"eq": 40.7},
			},
		},
	}, customerFixtureModel(), &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 || !strings.Contains(preds[0], "$.geo.lat") {
		t.Fatalf("unexpected predicate: %v", preds)
	}
}

func TestFilterBuilder_repeatedSubfield_jsonEach(t *testing.T) {
	var args []interface{}
	preds, err := FilterBuilder("x", map[string]interface{}{
		"addresses": map[string]interface{}{
			"name": map[string]interface{}{"eq": "Main"},
		},
	}, customerFixtureModel(), &args)
	if err != nil {
		t.Fatal(err)
	}
	if len(preds) != 1 {
		t.Fatalf("expected 1 predicate, got %v", preds)
	}
	if !strings.Contains(preds[0], "json_each") && !strings.Contains(preds[0], "customer__addresses") {
		t.Fatalf("expected json_each or child table, got %q", preds[0])
	}
}

func TestGeneratedColumnDDL_objectSubfield(t *testing.T) {
	ddl := GeneratedColumnDDL("customer", "address", []string{"name"})
	if !strings.Contains(ddl, "address__name") || !strings.Contains(ddl, "json_extract") {
		t.Fatalf("unexpected ddl: %s", ddl)
	}
}

func TestChildTableDDL_repeatedSubfield(t *testing.T) {
	ddl := ChildTableDDL("customer__addresses", []*models.FieldInfo{
		{Identifier: "name", FieldType: _const.TextField, InputType: _const.StringInput},
	}, true)
	if !strings.Contains(ddl, "parent_id") || !strings.Contains(ddl, "tenant_id") {
		t.Fatalf("unexpected ddl: %s", ddl)
	}
}
