package sqlquery

import (
	"fmt"
	"reflect"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	qutility "github.com/apito-io/open_driver/utility"
)

func isFilterComparator(key string) bool {
	switch key {
	case "between", "contains", "eq", "ne", "lt", "lte", "gt", "gtr", "in", "not_in":
		return true
	default:
		return false
	}
}

func findModelField(model *models.ModelType, identifier string) *models.FieldInfo {
	if model == nil {
		return nil
	}
	for _, f := range model.Fields {
		if f != nil && f.Identifier == identifier {
			return f
		}
	}
	return nil
}

func findSubField(parent *models.FieldInfo, identifier string) *models.FieldInfo {
	if parent == nil {
		return nil
	}
	for _, sf := range parent.SubFieldInfo {
		if sf != nil && sf.Identifier == identifier {
			return sf
		}
	}
	return nil
}

func filterFieldIsNested(filterMap map[string]interface{}) bool {
	for k := range filterMap {
		if !isFilterComparator(k) {
			return true
		}
	}
	return false
}

func nestedFilterBuilder(variable string, parentField *models.FieldInfo, filterMap map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}, jsonPath []string) ([]string, error) {
	if parentField == nil {
		return nil, fmt.Errorf("nested filter: nil parent field")
	}
	parentCol := parentField.Identifier
	if variable == "x" {
		parentCol = PhysicalSQLColumnForSystemRelationField(parentField.Identifier, modelType)
	}

	if parentField.FieldType == _const.RepeatedField {
		var conditions []string
		for key, val := range filterMap {
			if isFilterComparator(key) {
				continue
			}
			sub := findSubField(parentField, key)
			if sub == nil {
				return nil, fmt.Errorf("invalid field name %s in query", key)
			}
			subMap, ok := val.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid filter shape for field %s", key)
			}
			preds, err := repeatedSubfieldFilter(variable, parentCol, key, sub, subMap, modelType, sqlArgs)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, preds...)
		}
		return conditions, nil
	}

	var conditions []string
	for key, val := range filterMap {
		if isFilterComparator(key) {
			expr := nestedJSONExpr(variable, parentCol, jsonPath, parentField)
			preds, err := comparatorPredicates(expr, key, val, sqlArgs)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, preds...)
			continue
		}
		sub := findSubField(parentField, key)
		if sub == nil {
			return nil, fmt.Errorf("invalid field name %s in query", key)
		}
		subMap, ok := val.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid filter shape for field %s", key)
		}
		switch sub.FieldType {
		case _const.ObjectField:
			preds, err := nestedFilterBuilder(variable, sub, subMap, modelType, sqlArgs, append(jsonPath, key))
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, preds...)
		case _const.RepeatedField:
			preds, err := repeatedSubfieldFilter(variable, parentCol, key, sub, subMap, modelType, sqlArgs)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, preds...)
		default:
			path := append(jsonPath, key)
			expr := nestedJSONExpr(variable, parentCol, path, parentField)
			preds, err := leafSubfieldPredicates(expr, subMap, sqlArgs)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, preds...)
		}
	}
	return conditions, nil
}

func nestedJSONExpr(variable, parentCol string, jsonPath []string, parentField *models.FieldInfo) string {
	if parentField != nil && parentField.EnableIndexing && len(jsonPath) > 0 {
		gen := NestedGeneratedColumnName(parentField.Identifier, jsonPath...)
		return fmt.Sprintf("%s.%s", variable, gen)
	}
	path := JSONExtractPath(jsonPath...)
	return fmt.Sprintf("json_extract(%s.%s, '%s')", variable, parentCol, path)
}

func repeatedSubfieldFilter(variable, parentCol, subKey string, sub *models.FieldInfo, subMap map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) ([]string, error) {
	if modelType == nil {
		return nil, fmt.Errorf("model required for repeated filter")
	}
	childTable := ChildTableNameForRepeated(modelType.Name, parentCol)
	if sub != nil && sub.EnableIndexing {
		var inner []string
		for comp, val := range subMap {
			if !isFilterComparator(comp) {
				return nil, fmt.Errorf("invalid repeated subfilter for %s", subKey)
			}
			col := fmt.Sprintf("c.`%s`", subKey)
			preds, err := comparatorPredicates(col, comp, val, sqlArgs)
			if err != nil {
				return nil, err
			}
			inner = append(inner, preds...)
		}
		if len(inner) == 0 {
			return nil, nil
		}
		return []string{fmt.Sprintf(
			"EXISTS (SELECT 1 FROM `%s` AS c WHERE c.parent_id = %s.id AND %s)",
			childTable, variable, joinAnd(inner),
		)}, nil
	}
	inner, err := leafSubfieldPredicates(fmt.Sprintf("je.value ->> '$.%s'", subKey), subMap, sqlArgs)
	if err != nil {
		return nil, err
	}
	if len(inner) == 0 {
		return nil, nil
	}
	return []string{fmt.Sprintf(
		"EXISTS (SELECT 1 FROM json_each(%s.%s) AS je WHERE %s)",
		variable, parentCol, joinAnd(inner),
	)}, nil
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

func leafSubfieldPredicates(expr string, filterMap map[string]interface{}, sqlArgs *[]interface{}) ([]string, error) {
	var conditions []string
	for comp, val := range filterMap {
		if !isFilterComparator(comp) {
			return nil, fmt.Errorf("invalid nested filter comparator %q", comp)
		}
		preds, err := comparatorPredicates(expr, comp, val, sqlArgs)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, preds...)
	}
	return conditions, nil
}

func comparatorPredicates(expr, suffix string, value interface{}, sqlArgs *[]interface{}) ([]string, error) {
	if sqlArgs == nil {
		return nil, nil
	}
	var conditions []string
	switch suffix {
	case "contains":
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("contains requires string")
		}
		conditions = append(conditions, fmt.Sprintf(`%s LIKE ?`, expr))
		*sqlArgs = append(*sqlArgs, fmt.Sprintf("%%%s%%", s))
	case "eq", "ne", "lt", "lte", "gt", "gtr", "in", "not_in":
		op, ok := filterSQLComparator(suffix)
		if !ok {
			return nil, nil
		}
		switch suffix {
		case "in", "not_in":
			arr, ok := value.([]interface{})
			if !ok || len(arr) == 0 {
				if suffix == "in" {
					conditions = append(conditions, "0=1")
				} else {
					conditions = append(conditions, "1=1")
				}
				break
			}
			placeholders := trimRepeat("?,", len(arr))
			conditions = append(conditions, fmt.Sprintf(`%s %s (%s)`, expr, op, placeholders))
			for _, v := range arr {
				*sqlArgs = append(*sqlArgs, v)
			}
		default:
			conditions = append(conditions, fmt.Sprintf(`%s %s ?`, expr, op))
			*sqlArgs = append(*sqlArgs, value)
		}
	case "between":
		raw, err := parseBetweenRaw(value)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, fmt.Sprintf(`(%s >= ? AND %s <= ?)`, expr, expr))
		*sqlArgs = append(*sqlArgs, raw[0], raw[1])
	default:
		return nil, fmt.Errorf("unsupported comparator %s", suffix)
	}
	return conditions, nil
}

func trimRepeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out[:len(out)-1]
}

func applyTopLevelFieldFilter(variable string, field string, filterMap map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) ([]string, error) {
	fi := findModelField(modelType, field)
	if fi == nil {
		return nil, fmt.Errorf("invalid field name %s in query", field)
	}
	if filterFieldIsNested(filterMap) {
		switch fi.FieldType {
		case _const.ObjectField, _const.RepeatedField:
			return nestedFilterBuilder(variable, fi, filterMap, modelType, sqlArgs, nil)
		}
	}
	kind := qutility.GetUserFieldType(fi)
	sqlField := field
	if variable == "x" {
		sqlField = PhysicalSQLColumnForSystemRelationField(field, modelType)
	}
	fieldName := fmt.Sprintf("%s.%s", variable, sqlField)

	var conditions []string
	var actualValue interface{}
	skipKindCheck := false
	for suffix, value := range filterMap {
		actualValue = value
		preds, err := comparatorPredicates(fieldName, suffix, value, sqlArgs)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field, err)
		}
		if len(preds) > 0 {
			conditions = append(conditions, preds...)
			if suffix == "between" {
				skipKindCheck = true
			}
			continue
		}
		if suffix == "between" {
			raw, err := parseBetweenRaw(value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", field, err)
			}
			lo, err := coerceBetweenBound(kind, raw[0])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", field, err)
			}
			hi, err := coerceBetweenBound(kind, raw[1])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", field, err)
			}
			if sqlArgs != nil {
				conditions = append(conditions, fmt.Sprintf(`(%s >= ? AND %s <= ?)`, fieldName, fieldName))
				*sqlArgs = append(*sqlArgs, lo, hi)
			}
			skipKindCheck = true
		}
	}
	if !skipKindCheck && actualValue != nil {
		if k := getFieldType(actualValue); kind != k {
			return nil, fmt.Errorf("invalid value for %s in query: type mismatched", field)
		}
	}
	return conditions, nil
}

// unused import guard
var _ = reflect.Interface
