package sqlquery

import (
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/open_driver/project/sqlcommon"
)

// OneToOneUniqueIndexDrops lists idx_<table>_<fk>_unique indexes that must go when a relation is no longer 1:1.
// Column placement matches AddRelationFields.
func OneToOneUniqueIndexDrops(from, to *models.ConnectionType) []models.UniqueIndexDrop {
	if from == nil || to == nil {
		return nil
	}
	if from.Relation == "has_one" && to.Relation == "has_one" {
		return nil
	}
	toTable := utility.PhysicalSQLTableName(from.Model)
	fromTable := utility.PhysicalSQLTableName(to.Model)
	known := strings.TrimSpace(from.KnownAs)
	if known == "" {
		known = strings.TrimSpace(to.KnownAs)
	}
	toFK := relationFKColumnNameParts(to.Model, known)
	fromFK := relationFKColumnNameParts(from.Model, known)
	var drops []models.UniqueIndexDrop
	if toTable != "" && toFK != "" {
		drops = append(drops, models.UniqueIndexDrop{Table: toTable, Name: models.OneToOneUniqueIndexName(toTable, toFK)})
	}
	if fromTable != "" && fromFK != "" {
		drops = append(drops, models.UniqueIndexDrop{Table: fromTable, Name: models.OneToOneUniqueIndexName(fromTable, fromFK)})
	}
	return drops
}

// ApplyStaleOneToOne drops the has_one/has_one unique rule and leaves the foreign-key column in place.
func ApplyStaleOneToOne(engine string, from, to *models.ConnectionType, exec func(string) error) error {
	if err := ApplyUniqueKeyDDL(engine, false, nil, OneToOneUniqueIndexDrops(from, to), exec); err != nil {
		return err
	}
	for _, q := range staleOneToOneConstraintSQL(engine, from, to) {
		if q == "" {
			continue
		}
		if err := exec(q); err != nil && !IsIgnorableIndexDrop(err) {
			return err
		}
	}
	return nil
}

// staleOneToOneConstraintSQL drops a column UNIQUE constraint created by ADD COLUMN ... UNIQUE
// on Postgres/MySQL. The column and its values stay.
func staleOneToOneConstraintSQL(engine string, from, to *models.ConnectionType) []string {
	if from == nil || to == nil || (from.Relation == "has_one" && to.Relation == "has_one") {
		return nil
	}
	if sqlcommon.EngineIsSQLiteLike(engine) || strings.TrimSpace(engine) == "" {
		return nil
	}
	toTable := utility.PhysicalSQLTableName(from.Model)
	fromTable := utility.PhysicalSQLTableName(to.Model)
	known := strings.TrimSpace(from.KnownAs)
	if known == "" {
		known = strings.TrimSpace(to.KnownAs)
	}
	pairs := [][2]string{
		{toTable, relationFKColumnNameParts(to.Model, known)},
		{fromTable, relationFKColumnNameParts(from.Model, known)},
	}
	var out []string
	for _, pair := range pairs {
		table, col := pair[0], pair[1]
		if table == "" || col == "" {
			continue
		}
		if isPostgres(engine) {
			out = append(out, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s",
				sqlcommon.QuotePGIdent(table), sqlcommon.QuotePGIdent(table+"_"+col+"_key")))
			continue
		}
		out = append(out, fmt.Sprintf("ALTER TABLE %s DROP INDEX %s",
			sqlcommon.QuoteMySQLIdent(table), sqlcommon.QuoteMySQLIdent(col)))
	}
	return out
}

// ApplyUniqueKeyDDL drops removed composite indexes, then creates the keys still on the schema.
// A nil model list only drops. Duplicate rows surface as the database error.
func ApplyUniqueKeyDDL(engine string, sharedSaaS bool, modelsList []*models.ModelType, drops []models.UniqueIndexDrop, exec func(string) error) error {
	if exec == nil {
		return nil
	}
	for _, drop := range drops {
		q := dropIndexSQL(engine, drop.Table, drop.Name)
		if q == "" {
			continue
		}
		if err := exec(q); err != nil && !IsIgnorableIndexDrop(err) {
			return err
		}
	}
	for _, m := range modelsList {
		if m == nil || len(m.UniqueKeys) == 0 {
			continue
		}
		table := utility.PhysicalSQLTableName(m.Name)
		if table == "" {
			continue
		}
		prefixTenant := sharedSaaS && !models.ModelIsCommon(m) && !models.ModelIsTenantCatalogue(m)
		for _, key := range m.UniqueKeys {
			if key == nil || strings.TrimSpace(key.Name) == "" || len(key.Columns) == 0 {
				continue
			}
			cols := models.UniqueKeyIndexColumns(prefixTenant, key.Columns)
			q := createUniqueIndexSQL(engine, table, models.CompositeUniqueIndexName(table, key.Name), cols)
			if q == "" {
				continue
			}
			if err := exec(q); err != nil {
				return err
			}
		}
	}
	return nil
}

func createUniqueIndexSQL(engine, table, indexName string, columns []string) string {
	if table == "" || indexName == "" || len(columns) == 0 {
		return ""
	}
	if isPostgres(engine) {
		cols := make([]string, len(columns))
		for i, c := range columns {
			cols[i] = sqlcommon.QuotePGIdent(c)
		}
		return fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)",
			sqlcommon.QuotePGIdent(indexName), sqlcommon.QuotePGIdent(table), strings.Join(cols, ", "))
	}
	cols := make([]string, len(columns))
	for i, c := range columns {
		cols[i] = sqlcommon.QuoteMySQLIdent(c)
	}
	return fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s (%s)",
		sqlcommon.QuoteMySQLIdent(indexName), sqlcommon.QuoteMySQLIdent(table), strings.Join(cols, ", "))
}

func dropIndexSQL(engine, table, indexName string) string {
	if indexName == "" {
		return ""
	}
	if isPostgres(engine) {
		return "DROP INDEX IF EXISTS " + sqlcommon.QuotePGIdent(indexName)
	}
	if sqlcommon.EngineIsSQLiteLike(engine) || strings.TrimSpace(engine) == "" {
		return "DROP INDEX IF EXISTS " + sqlcommon.QuoteMySQLIdent(indexName)
	}
	if table == "" {
		return ""
	}
	return fmt.Sprintf("DROP INDEX IF EXISTS %s ON %s", sqlcommon.QuoteMySQLIdent(indexName), sqlcommon.QuoteMySQLIdent(table))
}

func isPostgres(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	return e == strings.ToLower(_const.PostgreSQLDriver) || e == "postgres"
}

// IsIgnorableIndexDrop reports a missing-index error from DROP INDEX.
func IsIgnorableIndexDrop(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such index") ||
		strings.Contains(s, "does not exist") ||
		strings.Contains(s, "doesn't exist") ||
		strings.Contains(s, "check that column/key exists") ||
		strings.Contains(s, "can't drop")
}
