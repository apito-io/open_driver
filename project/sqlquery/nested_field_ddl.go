package sqlquery

import (
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// NestedGeneratedColumnName returns the physical STORED column for an indexed object subpath.
func NestedGeneratedColumnName(parentField string, subPath ...string) string {
	parts := append([]string{strings.TrimSpace(parentField)}, subPath...)
	var clean []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "__")
}

// JSONExtractPath builds a SQLite json_extract path ($.a.b).
func JSONExtractPath(subPath ...string) string {
	if len(subPath) == 0 {
		return "$"
	}
	return "$." + strings.Join(subPath, ".")
}

// GeneratedColumnDDL returns ALTER TABLE ADD COLUMN ... AS (json_extract(...)) STORED for object subfields.
func GeneratedColumnDDL(tableName, parentColumn string, subPath []string) string {
	genCol := NestedGeneratedColumnName(parentColumn, subPath...)
	path := JSONExtractPath(subPath...)
	t := strings.ReplaceAll(tableName, "`", "``")
	pc := strings.ReplaceAll(parentColumn, "`", "``")
	gc := strings.ReplaceAll(genCol, "`", "``")
	return fmt.Sprintf(
		"ALTER TABLE `%s` ADD COLUMN `%s` AS (json_extract(`%s`, '%s')) STORED",
		t, gc, pc, path,
	)
}

// CreateIndexDDL builds CREATE INDEX IF NOT EXISTS for one or more columns.
func CreateIndexDDL(tableName, indexName string, columns ...string) string {
	t := strings.ReplaceAll(tableName, "`", "``")
	in := strings.ReplaceAll(indexName, "`", "``")
	cols := make([]string, len(columns))
	for i, c := range columns {
		cols[i] = "`" + strings.ReplaceAll(c, "`", "``") + "`"
	}
	return fmt.Sprintf("CREATE INDEX IF NOT EXISTS `%s` ON `%s` (%s)", in, t, strings.Join(cols, ", "))
}

// TenantFirstIndexColumns prefixes tenant_id for shared SaaS composite indexes.
func TenantFirstIndexColumns(sharedSaaS bool, columns ...string) []string {
	if !sharedSaaS {
		return columns
	}
	out := make([]string, 0, 1+len(columns))
	out = append(out, "tenant_id")
	out = append(out, columns...)
	return out
}

// ChildTableNameForRepeated returns `<table>__<repeated_field>` physical name.
func ChildTableNameForRepeated(modelName, repeatedField string) string {
	return utility.PhysicalSQLTableName(modelName) + "__" + strings.TrimSpace(repeatedField)
}

// ChildTableDDL creates a normalized search table for indexed repeated subfields.
func ChildTableDDL(tableName string, subfields []*models.FieldInfo, sharedSaaS bool) string {
	cols := []string{
		"id TEXT PRIMARY KEY",
		"parent_id TEXT NOT NULL",
		"idx INTEGER NOT NULL DEFAULT 0",
	}
	if sharedSaaS {
		cols = append(cols, "tenant_id TEXT")
	}
	for _, sf := range subfields {
		if sf == nil || strings.TrimSpace(sf.Identifier) == "" {
			continue
		}
		dt, err := fieldSQLDataType(sf)
		if err != nil {
			dt = "TEXT"
		}
		cols = append(cols, fmt.Sprintf("`%s` %s", sf.Identifier, dt))
	}
	t := strings.ReplaceAll(tableName, "`", "``")
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (%s)", t, strings.Join(cols, ", "))
}

// IndexedNestedSubfields collects subfields with EnableIndexing in object/repeated trees.
func IndexedNestedSubfields(f *models.FieldInfo, parentPath string) []indexedSubfield {
	if f == nil {
		return nil
	}
	var out []indexedSubfield
	switch f.FieldType {
	case _const.ObjectField, _const.RepeatedField:
		for _, sf := range f.SubFieldInfo {
			if sf == nil {
				continue
			}
			path := append([]string{}, splitFieldPath(parentPath)...)
			path = append(path, sf.Identifier)
			if sf.EnableIndexing {
				out = append(out, indexedSubfield{
					ParentField: f.Identifier,
					SubPath:     path,
					Field:       sf,
					Repeated:    f.FieldType == _const.RepeatedField,
				})
			}
			out = append(out, IndexedNestedSubfields(sf, strings.Join(path, ".")+".")...)
		}
	}
	return out
}

type indexedSubfield struct {
	ParentField string
	SubPath     []string
	Field       *models.FieldInfo
	Repeated    bool
}

func splitFieldPath(p string) []string {
	p = strings.Trim(p, ".")
	if p == "" {
		return nil
	}
	return strings.Split(p, ".")
}

// NestedIndexDDLStatements emits generated columns / child tables + indexes for a model.
func NestedIndexDDLStatements(model *models.ModelType, sharedSaaS bool) []string {
	if model == nil {
		return nil
	}
	table := utility.PhysicalSQLTableName(model.Name)
	var stmts []string
	for _, f := range model.Fields {
		if f == nil {
			continue
		}
		for _, idx := range IndexedNestedSubfields(f, "") {
			if idx.Repeated {
				child := ChildTableNameForRepeated(model.Name, idx.ParentField)
				stmts = append(stmts, ChildTableDDL(child, []*models.FieldInfo{idx.Field}, sharedSaaS))
				idxName := fmt.Sprintf("idx_%s_parent_%s", child, idx.Field.Identifier)
				cols := TenantFirstIndexColumns(sharedSaaS, "parent_id", idx.Field.Identifier)
				stmts = append(stmts, CreateIndexDDL(child, idxName, cols...))
				continue
			}
			relPath := idx.SubPath
			if len(relPath) > 0 && relPath[0] == idx.ParentField {
				relPath = relPath[1:]
			}
			genCol := NestedGeneratedColumnName(idx.ParentField, relPath...)
			stmts = append(stmts, GeneratedColumnDDL(table, idx.ParentField, relPath))
			idxName := fmt.Sprintf("idx_%s_%s", table, genCol)
			cols := TenantFirstIndexColumns(sharedSaaS, genCol)
			stmts = append(stmts, CreateIndexDDL(table, idxName, cols...))
		}
	}
	return stmts
}
