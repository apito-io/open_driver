package sqlquery

import "github.com/apito-io/engine/models"

// RelationEnsureNeeded reports whether a read should materialise relation artifacts
// and composite unique indexes. A published unique key must apply even when the
// query does not ask for a relation.
func RelationEnsureNeeded(param *models.CommonSystemParams) bool {
	if param == nil || len(param.ProjectSchemaModels) == 0 {
		return false
	}
	if param.ResolveParams != nil && param.ResolveParams.Args != nil {
		if conn, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok && len(conn) > 0 {
			return true
		}
	}
	for _, m := range param.ProjectSchemaModels {
		if m != nil && len(m.UniqueKeys) > 0 {
			return true
		}
	}
	return false
}
