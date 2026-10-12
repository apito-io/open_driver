package sqlquery

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

// IsJSONStoredListField reports whether a list field is stored as a JSON array column
// (dynamic / multi-select lists). Single-choice fixed dropdowns use plain TEXT.
func IsJSONStoredListField(f *models.FieldInfo) bool {
	if f == nil || f.FieldType != _const.ListField {
		return false
	}
	if f.Validation == nil {
		return true
	}
	if f.Validation.IsMultiChoice {
		return true
	}
	return len(f.Validation.FixedListElements) == 0
}

// FieldClassification groups model fields by storage shape for document transformation.
type FieldClassification struct {
	MultilineFields []string
	PictureField    []string
	GalleryField    []string
	ListFields      []string
	RepeatedFields  map[string][]*models.FieldInfo
	ObjectField     []string
	BooleanFields   []string
	DateFields      []string
}
