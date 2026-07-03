package sqlquery

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestIsJSONStoredListField_nilValidation(t *testing.T) {
	f := &models.FieldInfo{
		Identifier:   "subject_codes",
		FieldType:    "list",
		InputType:    "string",
		FieldSubType: "dynamicList",
	}
	if !IsJSONStoredListField(f) {
		t.Fatal("expected dynamic list without Validation to be JSON-stored")
	}
}

func TestIsJSONStoredListField_fixedDropdown(t *testing.T) {
	f := &models.FieldInfo{
		FieldType: "list",
		Validation: &models.Validation{
			FixedListElements: []interface{}{"a", "b"},
			IsMultiChoice:     false,
		},
	}
	if IsJSONStoredListField(f) {
		t.Fatal("expected single-choice fixed dropdown to use TEXT, not JSON list")
	}
}
