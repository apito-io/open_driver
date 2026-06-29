package sqlquery

import (
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"gitlab.com/apito.io/open_driver/project/sqlcommon"
)

// fieldSQLDataType maps Apito field metadata to a portable SQL column type for ALTER TABLE ... ADD.
func fieldSQLDataType(f *models.FieldInfo) (string, error) {
	if f == nil {
		return "", errors.New("nil FieldInfo")
	}
	switch f.FieldType {
	case _const.TextField:
		return "TEXT", nil
	case _const.MultilineField:
		return "TEXT", nil
	case _const.DateField:
		return "DATE", nil
	case _const.BooleanField:
		return "BOOLEAN", nil
	case _const.MediaField:
		if f.Validation != nil && f.Validation.IsGallery {
			return "JSON", nil
		}
		return "JSON", nil
	case _const.NumberField:
		switch f.InputType {
		case "int":
			return "INTEGER", nil
		case "double":
			return "NUMERIC", nil
		default:
			return "", fmt.Errorf("unsupported number input_type %q for SQL column", f.InputType)
		}
	case _const.ListField:
		if f.Validation != nil && len(f.Validation.FixedListElements) > 0 && !f.Validation.IsMultiChoice {
			return "TEXT", nil
		}
		return "JSON", nil
	case _const.RepeatedField, _const.ObjectField:
		return "JSON", nil
	default:
		return "", fmt.Errorf("unsupported FieldType %q for SQL column", f.FieldType)
	}
}

func fieldSQLValidations(f *models.FieldInfo) []string {
	if f == nil || f.Validation == nil {
		return nil
	}
	var validations []string
	if f.Validation.Required {
		var defaultValue interface{}
		switch f.InputType {
		case _const.StringInput:
			defaultValue = "''"
		case _const.IntInput:
			defaultValue = 0
		case _const.BoolInput:
			defaultValue = false
		case _const.DoubleInput:
			defaultValue = 0.0
		}
		validations = append(validations, fmt.Sprintf("NOT NULL DEFAULT %v", defaultValue))
	}
	if f.Validation.Unique {
		validations = append(validations, "UNIQUE")
	}
	return validations
}

// AlterTableAddFieldSQL returns DDL statements to add physical column(s) for one FieldInfo (locals expand to multiple columns).
// Engine must match DriverCredentials.Engine (e.g. postgresql, sqlite, libsql, mysql).
func AlterTableAddFieldSQL(engine, tableName string, f *models.FieldInfo) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	if f.InputType == "geo" {
		return nil, errors.New("geo Field is not supported in SQL DDL here")
	}
	datatype, err := fieldSQLDataType(f)
	if err != nil {
		return nil, err
	}
	validations := fieldSQLValidations(f)
	valSuffix := strings.TrimSpace(strings.Join(validations, " "))
	if valSuffix != "" {
		valSuffix = " " + valSuffix
	}

	eng := strings.ToLower(strings.TrimSpace(engine))

	if f.Validation != nil && len(f.Validation.Locals) > 0 {
		var out []string
		for _, local := range f.Validation.Locals {
			var column string
			if local != "en" {
				column = fmt.Sprintf(`%s_%s`, f.Identifier, local)
			} else {
				column = f.Identifier
			}
			switch eng {
			case _const.PostgreSQLDriver:
				tq := sqlcommon.QuotePGIdent(tableName)
				cq := sqlcommon.QuotePGIdent(column)
				out = append(out, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s%s`, tq, cq, datatype, valSuffix))
			case _const.MySQLDriver, _const.MariaDBDriver:
				t := strings.ReplaceAll(tableName, "`", "``")
				cq := sqlcommon.QuotePGIdent(column)
				out = append(out, fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN IF NOT EXISTS %s %s%s", t, cq, datatype, valSuffix))
			default:
				out = append(out, sqliteLikeAlterTableAddColumnStmts(tableName, column, datatype, valSuffix)...)
			}
		}
		return out, nil
	}

	switch eng {
	case _const.PostgreSQLDriver:
		tq := sqlcommon.QuotePGIdent(tableName)
		fq := sqlcommon.QuotePGIdent(f.Identifier)
		return []string{fmt.Sprintf(`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s%s`, tq, fq, datatype, valSuffix)}, nil
	case _const.MySQLDriver, _const.MariaDBDriver:
		t := strings.ReplaceAll(tableName, "`", "``")
		cq := sqlcommon.QuotePGIdent(f.Identifier) // reuse for MySQL/MariaDB
		return []string{fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN IF NOT EXISTS %s %s%s", t, cq, datatype, valSuffix)}, nil
	default:
		// SQLite / libsql / unknown: SQLite does NOT support UNIQUE in ALTER TABLE ADD COLUMN.
		// Strategy: add column without UNIQUE, then create a UNIQUE INDEX separately.
		return sqliteLikeAlterTableAddColumnStmts(tableName, f.Identifier, datatype, valSuffix), nil
	}
}

func sqliteLikeAlterTableAddColumnStmts(tableName, column, datatype, valSuffix string) []string {
	t := strings.ReplaceAll(tableName, "`", "``")
	cleanSuffix := strings.ReplaceAll(valSuffix, "UNIQUE", "")
	cleanSuffix = strings.TrimSpace(cleanSuffix)

	var stmts []string
	stmts = append(stmts, fmt.Sprintf("ALTER TABLE `%s` ADD %s %s%s", t, column, datatype, cleanSuffix))
	if strings.Contains(valSuffix, "UNIQUE") {
		idxName := fmt.Sprintf("idx_%s_%s", utility.PhysicalSQLTableName(tableName), column)
		idxQ := sqlcommon.QuotePGIdent(idxName)
		tableQ := strings.ReplaceAll(tableName, "`", "``")
		colQ := strings.ReplaceAll(column, "`", "``")
		stmts = append(stmts, fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(%s)", idxQ, tableQ, colQ))
	}
	return stmts
}

func isDuplicateSQLColumnError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "duplicate column") {
		return true
	}
	if strings.Contains(s, "already exists") {
		return true
	}
	// PostgreSQL duplicate_column
	if strings.Contains(s, "42701") {
		return true
	}
	return false
}

// ModelUserFieldColumnsDDL returns DDL statements to add physical columns for model fields.
func ModelUserFieldColumnsDDL(engine string, model *models.ModelType) ([]string, error) {
	if model == nil || len(model.Fields) == 0 {
		return nil, nil
	}
	tableName := utility.PhysicalSQLTableName(model.Name)
	var out []string
	for _, f := range model.Fields {
		if f == nil || strings.TrimSpace(f.Identifier) == "" {
			continue
		}
		if skipDDLSyntheticSystemRelationField(f, model) {
			continue
		}
		if f.InputType == "geo" {
			continue
		}
		stmts, err := AlterTableAddFieldSQL(engine, tableName, f)
		if err != nil {
			return nil, err
		}
		out = append(out, stmts...)
	}
	return out, nil
}

// legacy removed Driver.EnsureModelUserFieldColumns — use ModelUserFieldColumnsDDL + exec.
func _legacyEnsureModelUserFieldColumnsRemoved() {}
