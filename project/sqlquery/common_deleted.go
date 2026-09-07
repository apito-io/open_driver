package sqlquery

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
)

// CommonModelExcludeDeletedSQL hides soft-deleted common-model rows (meta.status).
func CommonModelExcludeDeletedSQL() string {
	return `(y.status IS NULL OR lower(COALESCE(y.status, '')) != 'deleted')`
}

// DocumentIsSoftDeleted reports whether a document's meta status is deleted.
func DocumentIsSoftDeleted(doc *types.DefaultDocumentStructure) bool {
	return doc != nil && doc.Meta != nil && strings.EqualFold(strings.TrimSpace(doc.Meta.Status), "deleted")
}

// HideDeletedCommonDocument returns nil when a common-model document is soft-deleted.
func HideDeletedCommonDocument(model *models.ModelType, doc *types.DefaultDocumentStructure) *types.DefaultDocumentStructure {
	if models.ModelIsCommon(model) && DocumentIsSoftDeleted(doc) {
		return nil
	}
	return doc
}
