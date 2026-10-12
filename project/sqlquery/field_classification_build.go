package sqlquery

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

// BuildFieldClassification groups model fields for CommonDocTransformation.
func BuildFieldClassification(fields []*models.FieldInfo) *FieldClassification {
	classification := &FieldClassification{RepeatedFields: map[string][]*models.FieldInfo{}}
	for _, f := range fields {
		if f == nil {
			continue
		}
		switch f.FieldType {
		case _const.MultilineField:
			classification.MultilineFields = append(classification.MultilineFields, f.Identifier)
		case _const.MediaField:
			if f.Validation != nil && f.Validation.IsGallery {
				classification.GalleryField = append(classification.GalleryField, f.Identifier)
			} else {
				classification.PictureField = append(classification.PictureField, f.Identifier)
			}
		case _const.ListField:
			if IsJSONStoredListField(f) {
				classification.ListFields = append(classification.ListFields, f.Identifier)
			}
		case _const.RepeatedField:
			classification.RepeatedFields[f.Identifier] = append(classification.RepeatedFields[f.Identifier], f.SubFieldInfo...)
		case _const.ObjectField:
			classification.ObjectField = append(classification.ObjectField, f.Identifier)
		case _const.BooleanField:
			classification.BooleanFields = append(classification.BooleanFields, f.Identifier)
		case _const.DateField:
			classification.DateFields = append(classification.DateFields, f.Identifier)
		}
	}
	return classification
}
