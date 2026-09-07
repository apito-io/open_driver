package sqlquery

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// ShadowRefSource is a tenant table/column that stores IDs of a common model.
type ShadowRefSource struct {
	Table  string
	Column string
}

// CommonModelShadowRefSources lists tenant FK and pivot columns that reference common.
func CommonModelShadowRefSources(schemaModels []*models.ModelType, common *models.ModelType) []ShadowRefSource {
	if common == nil || strings.TrimSpace(common.Name) == "" {
		return nil
	}
	var out []ShadowRefSource
	seen := map[string]struct{}{}
	add := func(table, column string) {
		table = strings.TrimSpace(table)
		column = strings.TrimSpace(column)
		if table == "" || column == "" {
			return
		}
		key := strings.ToLower(table) + "\x00" + strings.ToLower(column)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, ShadowRefSource{Table: table, Column: column})
	}
	commonName := common.Name
	for _, m := range schemaModels {
		if m == nil || models.ModelIsCommon(m) {
			continue
		}
		for _, c := range m.Connections {
			if c == nil || !relationEdgeMatchesParentModel(c, commonName) {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(c.Relation)) {
			case "has_one":
				add(utility.PhysicalSQLTableName(m.Name), relationFKColumnName(c))
			case "has_many":
				pivot := relationPivotTableNameParts(m.Name, commonName, c.KnownAs)
				add(pivot, utility.PhysicalSQLTableName(commonName)+"_id")
			}
		}
	}
	return out
}
