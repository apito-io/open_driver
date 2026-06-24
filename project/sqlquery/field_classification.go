package sqlquery

import "github.com/apito-io/engine/models"

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
