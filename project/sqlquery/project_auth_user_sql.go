package sqlquery

import (
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// ProjectAuthUserMetaSelectSQL returns meta column projections for the flat users auth table.
func ProjectAuthUserMetaSelectSQL() string {
	return "x.created_at AS sys_created_at, x.updated_at AS sys_updated_at, x.status AS sys_status"
}

// RelationDocumentMetaJoinSQL returns the meta join for relation target loads, or empty for auth users.
func RelationDocumentMetaJoinSQL(modelType *models.ModelType) string {
	if models.ModelIsProjectAuthUserModel(modelType) {
		return ""
	}
	return "LEFT JOIN meta AS z ON z.doc_id = x.id"
}

// ProjectAuthUserSelectColumns builds SELECT fragments for the auth users table (no meta join).
func ProjectAuthUserSelectColumns(local string, modelType *models.ModelType, returnCount bool) []string {
	if returnCount {
		return []string{"count(x.id)"}
	}
	var parts []string
	parts = append(parts, "x.id AS id")
	for _, f := range modelType.Fields {
		if f == nil {
			continue
		}
		phys := PhysicalSQLColumnForSystemRelationField(f.Identifier, modelType)
		switch phys {
		case "created_at", "updated_at", "status":
			continue
		}
		if f.Validation != nil && local != "en" && utility.ArrayContains(f.Validation.Locals, local) {
			parts = append(parts, fmt.Sprintf(`x.%s_%s AS %s`, phys, local, f.Identifier))
		} else {
			parts = append(parts, fmt.Sprintf(`x.%s AS %s`, phys, f.Identifier))
		}
	}
	parts = append(parts, ProjectAuthUserMetaSelectSQL())
	return parts
}

// ProjectAuthUserSingleDocQuery builds a SELECT for one auth user row by id.
func ProjectAuthUserSingleDocQuery(tableName string, local string, modelType *models.ModelType) string {
	cols := ProjectAuthUserSelectColumns(local, modelType, false)
	return fmt.Sprintf("SELECT %s FROM `%s` AS x WHERE x.id = ?", strings.Join(cols, ", "), tableName)
}

// RelationMetaJoinFragment returns a newline-prefixed meta join for relation SQL, or empty for auth users.
func RelationMetaJoinFragment(modelType *models.ModelType) string {
	j := RelationDocumentMetaJoinSQL(modelType)
	if j == "" {
		return ""
	}
	return "\n\t\t\t\t" + j
}

// RootListOrderBySQL returns the default list ORDER BY clause for RootResolverQueryBuilder.
func RootListOrderBySQL(modelType *models.ModelType) string {
	if models.ModelIsProjectAuthUserModel(modelType) {
		return `ORDER BY x.created_at DESC, x.id DESC`
	}
	return `ORDER BY y.created_at DESC, x.id DESC`
}
