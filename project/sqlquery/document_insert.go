package sqlquery

import (
	"encoding/json"
	"reflect"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// MapDocumentDataForSQLInsert normalizes doc.Data for SQL INSERT (JSON-encode list/repeated/gallery slices).
func MapDocumentDataForSQLInsert(docData map[string]interface{}, fields []*models.FieldInfo) (map[string]interface{}, error) {
	classification := BuildFieldClassification(fields)
	data := make(map[string]interface{}, len(docData))
	for k, v := range docData {
		if val, ok := v.(map[string]interface{}); ok {
			if html, ok := val["html"]; ok {
				data[k] = html
				continue
			}
		}
		switch reflect.ValueOf(v).Kind() {
		case reflect.Slice:
			if utility.ArrayContains(classification.GalleryField, k) ||
				utility.ArrayContains(classification.ListFields, k) ||
				len(classification.RepeatedFields[k]) > 0 {
				b, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				data[k] = string(b)
				continue
			}
		case reflect.Map:
			if utility.ArrayContains(classification.PictureField, k) {
				b, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				data[k] = string(b)
				continue
			}
		}
		data[k] = v
	}
	return data, nil
}
