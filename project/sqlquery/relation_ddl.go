package sqlquery

import (
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// RelationFieldsDDLStatements returns SQLite/D1 DDL for a relation pair (idempotent where supported).
func RelationFieldsDDLStatements(from, to *models.ConnectionType) []string {
	if from == nil || to == nil {
		return nil
	}
	toTableName := utility.PhysicalSQLTableName(from.Model)
	fromTableName := utility.PhysicalSQLTableName(to.Model)
	toFKColumn := relationFKColumnNameForModel(to.Model, from, to)
	fromFKColumn := relationFKColumnNameForModel(from.Model, from, to)
	pivotTable := relationPivotTableName(from, to)

	var stmts []string
	addColumn := func(tbl, col, refTbl string) {
		ft := strings.ReplaceAll(tbl, "`", "``")
		rt := strings.ReplaceAll(refTbl, "`", "``")
		stmts = append(stmts, fmt.Sprintf(
			"ALTER TABLE `%s` ADD COLUMN %s VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE",
			ft, col, rt))
	}
	uniqueIdx := func(tbl, col string) {
		ft := strings.ReplaceAll(tbl, "`", "``")
		stmts = append(stmts, fmt.Sprintf(
			"CREATE UNIQUE INDEX IF NOT EXISTS `idx_%s_%s_unique` ON `%s` (`%s`)",
			ft, col, ft, col))
	}

	switch from.Relation {
	case "has_one":
		switch to.Relation {
		case "has_one":
			addColumn(toTableName, toFKColumn, fromTableName)
			uniqueIdx(toTableName, toFKColumn)
			addColumn(fromTableName, fromFKColumn, toTableName)
			uniqueIdx(fromTableName, fromFKColumn)
		case "has_many":
			addColumn(fromTableName, fromFKColumn, toTableName)
		}
	case "has_many":
		switch to.Relation {
		case "has_many":
			pa, pb := sortedPhysicalPair(from.Model, to.Model)
			paq := strings.ReplaceAll(pa, "`", "``")
			pbq := strings.ReplaceAll(pb, "`", "``")
			escPivot := strings.ReplaceAll(pivotTable, "`", "``")
			stmts = append(stmts, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+"`%s`"+`(
				`+"`%s_id`"+` VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				`+"`%s_id`"+` VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				PRIMARY KEY (`+"`%s_id`, `%s_id`"+`)
			)`, escPivot, paq, paq, pbq, pbq, paq, pbq))
			stmts = append(stmts,
				fmt.Sprintf("CREATE INDEX IF NOT EXISTS `idx_%s_%s_id` ON `%s` (`%s_id`)", escPivot, paq, escPivot, paq),
				fmt.Sprintf("CREATE INDEX IF NOT EXISTS `idx_%s_%s_id` ON `%s` (`%s_id`)", escPivot, pbq, escPivot, pbq),
			)
		case "has_one":
			addColumn(toTableName, toFKColumn, fromTableName)
		}
	}
	return stmts
}
