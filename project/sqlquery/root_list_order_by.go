package sqlquery

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// RootListOrderByFromArgs builds ORDER BY from GraphQL `sort` args when present.
// Only FieldIsSortable model fields are applied. Falls back to RootListOrderBySQL.
func RootListOrderByFromArgs(param *models.CommonSystemParams) string {
	if param == nil || param.Model == nil {
		return RootListOrderBySQL(nil)
	}
	if param.ResolveParams == nil || param.ResolveParams.Args == nil {
		return RootListOrderBySQL(param.Model)
	}
	raw, ok := param.ResolveParams.Args["sort"].(map[string]interface{})
	if !ok || len(raw) == 0 {
		return RootListOrderBySQL(param.Model)
	}

	byID := make(map[string]*models.FieldInfo, len(param.Model.Fields))
	for _, f := range param.Model.Fields {
		if f == nil || strings.TrimSpace(f.Identifier) == "" {
			continue
		}
		byID[f.Identifier] = f
		if strings.HasPrefix(f.Identifier, "system_") {
			byID[utility.CanonicalSystemRelationFieldIdentifier(f.Identifier)] = f
		}
	}

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		f := byID[key]
		if f == nil || !utility.FieldIsSortable(f) {
			continue
		}
		dir := normalizeSortDirection(raw[key])
		if dir == "" {
			continue
		}
		phys := PhysicalSQLColumnForSystemRelationField(f.Identifier, param.Model)
		if phys == "" || strings.ContainsAny(phys, "`\"'; \t\n") {
			continue
		}
		parts = append(parts, fmt.Sprintf("`x`.`%s` %s", strings.ReplaceAll(phys, "`", "``"), dir))
	}
	if len(parts) == 0 {
		return RootListOrderBySQL(param.Model)
	}
	// Stable tie-break so pagination is deterministic.
	parts = append(parts, "`x`.`id` ASC")
	return "ORDER BY " + strings.Join(parts, ", ")
}

func normalizeSortDirection(v interface{}) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ASC":
		return "ASC"
	case "DESC":
		return "DESC"
	default:
		return ""
	}
}
