package sqlite

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/helper"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	"github.com/tailor-platform/graphql"
	qutility "gitlab.com/apito.io/open_driver/utility"
	"gitlab.com/apito.io/open_driver/project/sqlquery"
)

// mergeGroupedSQLPredicates builds `(pred AND …)` groups from ConditionBuilder / hook output.
// Skips empty slices so we never emit invalid SQL like `WHERE ()` when `where.AND` is empty or
// every FilterBuilder branch produced no predicate (e.g. unsupported comparator).
func mergeGroupedSQLPredicates(filters map[string][]string) []string {
	var merged []string
	for condition, preds := range filters {
		if len(preds) == 0 {
			continue
		}
		merged = append(merged, fmt.Sprintf(`(%s)`, strings.Join(preds, fmt.Sprintf(` %s `, condition))))
	}
	return merged
}

// relationEdgeMatchesParentModel reports whether a connection edge points at parentModel (normalized SQL table id).
func relationEdgeMatchesParentModel(edge *models.ConnectionType, parentModel string) bool {
	if edge == nil || parentModel == "" || edge.Model == "" {
		return false
	}
	return utility.PhysicalSQLTableName(edge.Model) == utility.PhysicalSQLTableName(parentModel)
}

// stripDuplicateGoTimeOffset removes a duplicated zone suffix from legacy Go time.Time.String() values
// (e.g. "2026-05-27 11:28:01 +0600 +0600" → "2026-05-27 11:28:01 +0600").
func stripDuplicateGoTimeOffset(s string) string {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return s
	}
	last := parts[len(parts)-1]
	prev := parts[len(parts)-2]
	if last == prev && len(last) == 5 && (strings.HasPrefix(last, "+") || strings.HasPrefix(last, "-")) {
		return strings.Join(parts[:len(parts)-1], " ")
	}
	return s
}

// formatSQLMetaTimestamp normalizes meta.created_at / meta.updated_at (and similar) from SQL row scans.
// SQLite and libsql commonly return DATE/DATETIME as string; PostgreSQL returns time.Time.
// All API-facing timestamps are emitted as UTC RFC3339.
func formatSQLMetaTimestamp(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339), nil
	case *time.Time:
		if t == nil {
			return "", nil
		}
		return t.UTC().Format(time.RFC3339), nil
	case string:
		s := strings.TrimSpace(stripDuplicateGoTimeOffset(t))
		if s == "" {
			return "", nil
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05 -0700 MST", s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05 -0700", s); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		if parsed, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
			return parsed.UTC().Format(time.RFC3339), nil
		}
		return s, nil
	case []byte:
		return formatSQLMetaTimestamp(string(t))
	default:
		return "", fmt.Errorf("unexpected type for meta timestamp: %T", v)
	}
}

// sqlMetaString coerces nullable SQL meta columns (status, created_by, updated_by) from row scans.
func sqlMetaString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

type QueryBuilderParam struct {
	CollectionName string
	RelationName   string
	Args           map[string]interface{}
	ParentModel    string
	ModelType      *models.ModelType
}

var FilterSuffix = map[string]string{
	"eq":     "==",
	"ne":     "!=",
	"lt":     "<",
	"lte":    "<=",
	"gt":     ">",
	"gte":    ">=",
	"in":     "IN",
	"not_in": "NOT IN",
}

// filterSQLComparator maps GraphQL filter suffixes to SQL operators for the parameterized query path.
func filterSQLComparator(suffix string) (string, bool) {
	switch suffix {
	case "eq":
		return "=", true
	case "ne":
		return "!=", true
	case "lt":
		return "<", true
	case "lte":
		return "<=", true
	case "gt":
		return ">", true
	case "gte", "gtr":
		return ">=", true
	case "in":
		return "IN", true
	case "not_in":
		return "NOT IN", true
	default:
		return "", false
	}
}

func SelectBuilder(mv string, local string, modelType *models.ModelType, returnCount bool) []string {

	if models.ModelIsProjectAuthUserModel(modelType) {
		return sqlquery.ProjectAuthUserSelectColumns(local, modelType, returnCount)
	}

	var returnType []string
	if returnCount {
		return []string{"count(x.id)"}
	}

	var dataJson []string
	for _, f := range modelType.Fields {
		phys := PhysicalSQLColumnForSystemRelationField(f.Identifier, modelType)
		if f.Validation != nil && local != "en" && utility.ArrayContains(f.Validation.Locals, local) {
			dataJson = append(dataJson, fmt.Sprintf(`x.%s_%s AS %s`, phys, local, f.Identifier))
		} else {
			dataJson = append(dataJson, fmt.Sprintf(`x.%s AS %s`, phys, f.Identifier))
		}
	}
	// Avoid `SELECT x.id AS id, , y.created_at...` when the model has no user fields yet.
	returnType = append(returnType, "x.id AS id")
	if len(dataJson) > 0 {
		returnType = append(returnType, strings.Join(dataJson, ", "))
	}

	metaQuery := fmt.Sprintf(`%s.created_at AS sys_created_at, %s.updated_at AS sys_updated_at, %s.created_by AS sys_created_by, %s.updated_by AS sys_updated_by, %s.status as sys_status`, mv, mv, mv, mv, mv)
	returnType = append(returnType, metaQuery)

	return returnType
}

// aggregateFieldNamesFromSelection mirrors Arango ReturnAggregateBuilder: fields under
// selection set path aggregate { fieldA fieldB }.
func aggregateFieldNamesFromSelection(param *models.CommonSystemParams) []string {
	if param == nil || param.QuerySelectionSets == nil {
		return nil
	}
	fields := helper.ReturnAggregateBuilder("x", *param.QuerySelectionSets)
	if len(fields) == 0 {
		return nil
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AggregateSelectExpressions builds COALESCE(SUM(x.col), 0) AS field for numeric model fields
// requested in the GraphQL aggregate selection (same field set Arango SUMs).
func AggregateSelectExpressions(modelType *models.ModelType, fieldNames []string) ([]string, error) {
	if modelType == nil {
		return nil, errors.New("model is required for aggregate query")
	}
	byID := make(map[string]*models.FieldInfo, len(modelType.Fields))
	for _, f := range modelType.Fields {
		if f == nil || f.Identifier == "" {
			continue
		}
		byID[f.Identifier] = f
	}
	var exprs []string
	for _, name := range fieldNames {
		name = strings.TrimSpace(name)
		if name == "" || name == "__typename" {
			continue
		}
		f, ok := byID[name]
		if !ok {
			return nil, fmt.Errorf("aggregate field %q is not on model %q", name, modelType.Name)
		}
		switch f.InputType {
		case _const.IntInput, _const.DoubleInput:
		default:
			return nil, fmt.Errorf("aggregate field %q on model %q is not numeric", name, modelType.Name)
		}
		phys := PhysicalSQLColumnForSystemRelationField(f.Identifier, modelType)
		// COALESCE keeps GraphQL from returning null when no rows / all-null column.
		exprs = append(exprs, fmt.Sprintf("COALESCE(SUM(`x`.`%s`), 0) AS `%s`", phys, f.Identifier))
	}
	if len(exprs) == 0 {
		return nil, errors.New("no aggregate fields selected")
	}
	return exprs, nil
}

// GroupByKey is one `groupBy: [{ key, value }]` entry, kept in the order the client sent
// them so `group_by_identifier_1` always maps to the first key.
type GroupByKey struct {
	Key   string
	Value string
}

// GroupByKeysFromArgs reads the ordered groupBy argument off the GraphQL resolve params.
func GroupByKeysFromArgs(param *models.CommonSystemParams) []GroupByKey {
	if param == nil || param.ResolveParams == nil || param.ResolveParams.Args == nil {
		return nil
	}
	raw, ok := param.ResolveParams.Args["groupBy"].([]interface{})
	if !ok {
		return nil
	}
	var keys []GroupByKey
	for _, entry := range raw {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		value, _ := m["value"].(string)
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key, _ := m["key"].(string)
		keys = append(keys, GroupByKey{Key: strings.TrimSpace(key), Value: value})
	}
	return keys
}

// GroupBySelectExpressions maps each groupBy value onto its physical column and aliases it
// as group_by_identifier_N, matching the Arango builder's RETURN shape.
func GroupBySelectExpressions(modelType *models.ModelType, keys []GroupByKey) (selectExprs []string, groupCols []string, err error) {
	if modelType == nil {
		return nil, nil, errors.New("model is required for group by query")
	}
	if len(keys) > groupByIdentifierLimit {
		return nil, nil, fmt.Errorf("group by supports at most %d keys", groupByIdentifierLimit)
	}
	for i, k := range keys {
		phys := PhysicalSQLColumnForSystemRelationField(k.Value, modelType)
		if !groupByColumnExists(modelType, k.Value, phys) {
			return nil, nil, fmt.Errorf("group by field %q is not on model %q", k.Value, modelType.Name)
		}
		col := fmt.Sprintf("`x`.`%s`", phys)
		groupCols = append(groupCols, col)
		selectExprs = append(selectExprs, fmt.Sprintf("%s AS `%s`", col, GroupByIdentifierAlias(i+1)))
	}
	return selectExprs, groupCols, nil
}

// groupByKeysUseRelationColumn reports whether any key targets a relation-derived
// system_<model>_id column rather than a plain model field.
func groupByKeysUseRelationColumn(keys []GroupByKey) bool {
	for _, k := range keys {
		id := utility.CanonicalSystemRelationFieldIdentifier(strings.TrimSpace(k.Value))
		if strings.HasPrefix(id, "system_") && strings.HasSuffix(id, "_id") {
			return true
		}
	}
	return false
}

// GroupByIdentifierAlias is the GraphQL field name for the Nth group key.
func GroupByIdentifierAlias(n int) string {
	return fmt.Sprintf("group_by_identifier_%d", n)
}

// IsGroupByIdentifier reports whether a selected field is a group key placeholder rather
// than a numeric model field to SUM.
func IsGroupByIdentifier(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "group_by_identifier_")
}

const groupByIdentifierLimit = 2

// groupByColumnExists accepts either a declared model field or a relation-derived
// system_<model>_id column, which has no FieldInfo entry but is a real FK column.
func groupByColumnExists(modelType *models.ModelType, identifier string, physical string) bool {
	if physical != identifier {
		// PhysicalSQLColumnForSystemRelationField only rewrites identifiers it resolved
		// against the model's connections, so a rewrite means the FK column exists.
		return true
	}
	for _, f := range modelType.Fields {
		if f != nil && f.Identifier == identifier {
			return true
		}
	}
	return false
}

func graphqlArgInt(m map[string]interface{}, key string, def int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

// graphqlArgBool coerces GraphQL / JSON variable shapes into a boolean (used for intersect).
func graphqlArgBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case *bool:
		if t == nil {
			return false
		}
		return *t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case json.Number:
		i, err := t.Int64()
		return err == nil && i != 0
	default:
		return false
	}
}

func LimitBuilder(param *graphql.ResolveParams) (int, int) {
	arg := param.Args

	limit := graphqlArgInt(arg, "limit", 10)
	start := graphqlArgInt(arg, "start", 0)
	page := graphqlArgInt(arg, "page", 1)

	if page > 1 {
		offset := limit * (page - 1)
		return limit, offset
	}
	return limit, start
}

func getFieldType(val interface{}) reflect.Kind {
	return reflect.TypeOf(val).Kind()
}

// parseBetweenRaw extracts two boundary values from a GraphQL `between` filter (parity with Arango query builder).
func parseBetweenRaw(value interface{}) ([2]interface{}, error) {
	var out [2]interface{}
	switch v := value.(type) {
	case []interface{}:
		if len(v) < 2 {
			return out, errors.New("between requires two values")
		}
		out[0], out[1] = v[0], v[1]
	case []string:
		if len(v) < 2 {
			return out, errors.New("between requires two values")
		}
		out[0], out[1] = v[0], v[1]
	case string:
		parts := strings.Split(v, ",")
		if len(parts) < 2 {
			return out, errors.New("between string must contain two comma-separated values")
		}
		out[0] = strings.TrimSpace(parts[0])
		out[1] = strings.TrimSpace(parts[1])
	default:
		return out, fmt.Errorf("between value type not supported: %T", value)
	}
	return out, nil
}

func sqlLiteralForFilter(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(t, "'", "''"))
	case int, int32, int64, float64, bool:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(fmt.Sprint(t), "'", "''"))
	}
}

// coerceBetweenBound maps a single `between` endpoint to a bindable value for the field's logical type.
func coerceBetweenBound(kind reflect.Kind, v interface{}) (interface{}, error) {
	switch kind {
	case reflect.String, reflect.Interface:
		switch t := v.(type) {
		case string:
			return t, nil
		case float64:
			return fmt.Sprint(t), nil
		case int, int32, int64:
			return fmt.Sprint(t), nil
		default:
			return nil, fmt.Errorf("between bound must be string-like for this field, got %T", v)
		}
	case reflect.Int:
		switch t := v.(type) {
		case int:
			return t, nil
		case int32:
			return int(t), nil
		case int64:
			return int(t), nil
		case float64:
			return int(t), nil
		default:
			return nil, fmt.Errorf("between bound must be numeric for int field, got %T", v)
		}
	case reflect.Float64:
		switch t := v.(type) {
		case float64:
			return t, nil
		case int:
			return float64(t), nil
		case int32:
			return float64(t), nil
		case int64:
			return float64(t), nil
		default:
			return nil, fmt.Errorf("between bound must be numeric for float field, got %T", v)
		}
	default:
		return v, nil
	}
}

// validateListFilterElements type-checks each member of an `in` / `not_in` list
// against the field kind, applying the same numeric/string leniency the
// single-value check uses (JSON numbers arrive as float64).
func validateListFilterElements(field string, kind reflect.Kind, value interface{}) error {
	arr, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("invalid value for %s in query: expected a list", field)
	}
	for _, item := range arr {
		if item == nil {
			continue
		}
		k := getFieldType(item)
		if k == kind {
			continue
		}
		switch {
		case kind == reflect.Int && k == reflect.Float64:
			if f, ok := item.(float64); ok && f == float64(int64(f)) {
				continue
			}
		case kind == reflect.Int && k == reflect.String:
			if s, ok := item.(string); ok {
				if _, err := strconv.ParseInt(s, 10, 64); err == nil {
					continue
				}
			}
		case kind == reflect.Float64 && (k == reflect.Int || k == reflect.Int64 || k == reflect.Int32):
			continue
		}
		return fmt.Errorf("invalid value for %s in query: type mismatched", field)
	}
	return nil
}

func ConditionBuilder(variable string, args map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) (map[string][]string, error) {

	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = qutility.GetUserFieldType(field)
	}

	conditions := make(map[string][]string)

	if w, ok := args["where"]; args["where"] != nil && ok && len(w.(map[string]interface{})) > 0 {
		where := w.(map[string]interface{})
		for field, filterObj := range where {

			switch field { // default is x.data
			case "role":
				variable = "x"
			}

			// if AND / OR found
			switch field {
			case "AND":
				preds, err := FilterBuilder(variable, filterObj.(map[string]interface{}), modelType, sqlArgs)
				if err != nil {
					return nil, err
				}
				conditions["AND"] = append(conditions["AND"], preds...)
			case "OR":
				preds, err := FilterBuilder(variable, filterObj.(map[string]interface{}), modelType, sqlArgs)
				if err != nil {
					return nil, err
				}
				conditions["OR"] = append(conditions["OR"], preds...)
			default:
				sub := map[string]interface{}{field: filterObj}
				preds, err := FilterBuilder(variable, sub, modelType, sqlArgs)
				if err != nil {
					return nil, err
				}
				conditions["AND"] = append(conditions["AND"], preds...)
			}
		}
	}

	return conditions, nil
}

func FilterBuilder(variable string, where map[string]interface{}, modelType *models.ModelType, sqlArgs *[]interface{}) ([]string, error) {

	userDefinedFieldNames := make(map[string]reflect.Kind)
	for _, field := range modelType.Fields {
		userDefinedFieldNames[field.Identifier] = qutility.GetUserFieldType(field)
	}

	var conditions []string

	for field, filterObj := range where {

		kind, fieldOK := userDefinedFieldNames[field]
		if !fieldOK {
			return nil, fmt.Errorf("invalid field name %s in query", field)
		}

		filterMap, ok := filterObj.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid filter shape for field %s", field)
		}

		sqlField := field
		// SQL row filters use table alias `x`; Arango-style paths use `x.data` — keep document keys intact there.
		if variable == "x" {
			sqlField = PhysicalSQLColumnForSystemRelationField(field, modelType)
		}
		fieldName := fmt.Sprintf("%s.%s", variable, sqlField)

		var actualValue interface{}
		skipKindCheck := false

		for suffix, value := range filterMap {

			actualValue = value

			switch suffix {
			case "between":
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
				} else {
					conditions = append(conditions, fmt.Sprintf(`(%s >= %s AND %s <= %s)`,
						fieldName, sqlLiteralForFilter(lo), fieldName, sqlLiteralForFilter(hi)))
				}
				skipKindCheck = true
			case "contains":
				if sqlArgs != nil {
					conditions = append(conditions, fmt.Sprintf(`%s LIKE ?`, fieldName))
					*sqlArgs = append(*sqlArgs, fmt.Sprintf("%%%s%%", value.(string)))
				} else {
					conditions = append(conditions, fmt.Sprintf(`%s LIKE '%%%s%%'`, fieldName, value.(string)))
				}
			case "eq", "ne", "lt", "lte", "gt", "gtr", "in", "not_in":
				if suffix == "in" || suffix == "not_in" {
					// Value is a list; the trailing check compares a single value
					// against the field kind, so validate the elements here instead.
					if err := validateListFilterElements(field, kind, value); err != nil {
						return nil, err
					}
					skipKindCheck = true
				}
				if sqlArgs != nil {
					op, ok := filterSQLComparator(suffix)
					if !ok {
						continue
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
						placeholders := strings.TrimSuffix(strings.Repeat("?,", len(arr)), ",")
						conditions = append(conditions, fmt.Sprintf(`%s %s (%s)`, fieldName, op, placeholders))
						for _, v := range arr {
							*sqlArgs = append(*sqlArgs, v)
						}
					case "ne":
						conditions = append(conditions, fmt.Sprintf(`(%s IS NULL OR %s %s ?)`, fieldName, fieldName, op))
						*sqlArgs = append(*sqlArgs, value)
					default:
						conditions = append(conditions, fmt.Sprintf(`%s %s ?`, fieldName, op))
						*sqlArgs = append(*sqlArgs, value)
					}
				} else {
					switch value := value.(type) {
					case int, float64, bool:
						if suffix == "ne" {
							conditions = append(conditions, fmt.Sprintf(`(%s IS NULL OR %s %s %v)`, fieldName, fieldName, FilterSuffix[suffix], value))
						} else {
							conditions = append(conditions, fmt.Sprintf(`%s %s %v`, fieldName, FilterSuffix[suffix], value))
						}
					case string:
						if suffix == "ne" {
							conditions = append(conditions, fmt.Sprintf(`(%s IS NULL OR %s %s '%v')`, fieldName, fieldName, FilterSuffix[suffix], value))
						} else {
							conditions = append(conditions, fmt.Sprintf(`%s %s '%v'`, fieldName, FilterSuffix[suffix], value))
						}
					case []interface{}:
						var vals []string
						for _, v := range value {
							switch v.(type) {
							case int, float64:
								vals = append(vals, fmt.Sprintf("%v", v))
							case string:
								vals = append(vals, fmt.Sprintf("'%v'", v))
							}
						}
						final := fmt.Sprintf("[%s]", strings.Join(vals, ","))
						conditions = append(conditions, fmt.Sprintf(`COUNT(%s[* FILTER CONTAINS(%s, CURRENT)])`, fieldName, final))
					}
				}
			}
		}

		if !skipKindCheck {
			if k := getFieldType(actualValue); kind != k {
				// GraphQL/JSON numbers decode as float64; accept whole floats for int fields.
				// Also accept numeric strings for int fields (common client coercion).
				compatible := false
				switch {
				case kind == reflect.Int && k == reflect.Float64:
					if f, ok := actualValue.(float64); ok && f == float64(int64(f)) {
						compatible = true
					}
				case kind == reflect.Int && k == reflect.String:
					if s, ok := actualValue.(string); ok {
						if _, err := strconv.ParseInt(s, 10, 64); err == nil {
							compatible = true
						}
					}
				case kind == reflect.Float64 && (k == reflect.Int || k == reflect.Int64 || k == reflect.Int32):
					compatible = true
				}
				if !compatible {
					return nil, fmt.Errorf("invalid value for %s in query: type mismatched", field)
				}
			}
		}

	}

	return conditions, nil
}

func publicRelationConnectionForKey(model *models.ModelType, relationKey string) *models.ConnectionType {
	if model == nil {
		return nil
	}
	relationKey = strings.TrimSpace(relationKey)
	for _, conn := range model.Connections {
		if conn == nil {
			continue
		}
		if strings.TrimSpace(conn.KnownAs) == relationKey {
			return conn
		}
		if utility.ModelIDMatchesGraphQLField(conn.Model, relationKey) {
			return conn
		}
		if utility.SingularResourceName(conn.Model) == relationKey {
			return conn
		}
	}
	return nil
}

func relationIDFilterPredicates(alias string, filterMap map[string]interface{}, sqlArgs *[]interface{}) ([]string, error) {
	idFilter, ok := filterMap["_id"]
	if !ok || idFilter == nil {
		return nil, nil
	}
	idMap, ok := idFilter.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid relation _id filter shape")
	}
	var preds []string
	for suffix, value := range idMap {
		op, ok := filterSQLComparator(suffix)
		if !ok {
			continue
		}
		switch suffix {
		case "in", "not_in":
			arr, ok := value.([]interface{})
			if !ok || len(arr) == 0 {
				if suffix == "in" {
					preds = append(preds, "0=1")
				} else {
					preds = append(preds, "1=1")
				}
				continue
			}
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(arr)), ",")
			preds = append(preds, fmt.Sprintf(`%s.id %s (%s)`, alias, op, placeholders))
			if sqlArgs != nil {
				for _, v := range arr {
					*sqlArgs = append(*sqlArgs, v)
				}
			}
		default:
			preds = append(preds, fmt.Sprintf(`%s.id %s ?`, alias, op))
			if sqlArgs != nil {
				*sqlArgs = append(*sqlArgs, value)
			}
		}
	}
	return preds, nil
}

func relatedModelFilterPredicates(alias string, filterMap map[string]interface{}, relatedModel *models.ModelType, sqlArgs *[]interface{}) ([]string, error) {
	if len(filterMap) == 0 {
		return nil, nil
	}
	var preds []string
	idPreds, err := relationIDFilterPredicates(alias, filterMap, sqlArgs)
	if err != nil {
		return nil, err
	}
	preds = append(preds, idPreds...)
	fieldFilters := make(map[string]interface{}, len(filterMap))
	for k, v := range filterMap {
		if k == "_id" {
			continue
		}
		fieldFilters[k] = v
	}
	if len(fieldFilters) > 0 {
		fieldPreds, err := FilterBuilder(alias, fieldFilters, relatedModel, sqlArgs)
		if err != nil {
			return nil, err
		}
		preds = append(preds, fieldPreds...)
	}
	return preds, nil
}

func publicRelationFilterPredicates(param *models.CommonSystemParams, sqlArgs *[]interface{}) ([]string, error) {
	if param == nil || param.ResolveParams == nil || param.Model == nil {
		return nil, nil
	}
	relationArg, ok := param.ResolveParams.Args["relation"].(map[string]interface{})
	if !ok || len(relationArg) == 0 {
		return nil, nil
	}
	if len(param.ProjectSchemaModels) == 0 {
		return nil, errors.New("project schema models are required for relation filters")
	}

	var preds []string
	for relationKey, rawFilter := range relationArg {
		filterMap, ok := rawFilter.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid relation filter shape for %s", relationKey)
		}
		conn := publicRelationConnectionForKey(param.Model, relationKey)
		if conn == nil {
			return nil, fmt.Errorf("relation %q not found on model %q", relationKey, param.Model.Name)
		}
		relatedModel := findSchemaModel(param.ProjectSchemaModels, conn.Model)
		if relatedModel == nil {
			return nil, fmt.Errorf("relation model %q not found for %q", conn.Model, relationKey)
		}

		alias := fmt.Sprintf("rel_%d", len(preds))
		relatedPreds, err := relatedModelFilterPredicates(alias, filterMap, relatedModel, sqlArgs)
		if err != nil {
			return nil, err
		}
		if len(relatedPreds) == 0 {
			continue
		}

		knownAs := strings.TrimSpace(conn.KnownAs)
		relatedTable := utility.PhysicalSQLTableName(relatedModel.Name)
		listedFK, listedHasFK := fkPhysicalColumnOnModelToTarget(param.Model, relatedModel.Name, knownAs)
		relatedFK, relatedHasFK := fkPhysicalColumnOnModelToTarget(relatedModel, param.Model.Name, knownAs)
		inferredListedFK := relationFKColumnNameParts(relatedModel.Name, knownAs)
		inferredRelatedFK := relationFKColumnNameParts(param.Model.Name, knownAs)
		isManyToMany := sqlConnectionIsTrueManyToMany(param.ProjectSchemaModels, param.Model.Name, relatedModel.Name)
		if !listedHasFK && conn.Relation == "has_one" && inferredListedFK != "" {
			listedFK = inferredListedFK
			listedHasFK = true
		}
		if !relatedHasFK && conn.Relation == "has_many" && !isManyToMany && inferredRelatedFK != "" {
			relatedFK = inferredRelatedFK
			relatedHasFK = true
		}
		switch {
		case listedHasFK:
			preds = append(preds, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM `%s` AS %s WHERE %s.id = `x`.`%s` AND %s)",
				relatedTable,
				alias,
				alias,
				listedFK,
				strings.Join(relatedPreds, " AND "),
			))
		case isManyToMany:
			pivotAlias := fmt.Sprintf("relp_%d", len(preds))
			pivotTable := relationPivotTableNameParts(param.Model.Name, relatedModel.Name, knownAs)
			listedPivotCol := utility.PhysicalSQLTableName(param.Model.Name) + "_id"
			relatedPivotCol := utility.PhysicalSQLTableName(relatedModel.Name) + "_id"
			preds = append(preds, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM `%s` AS %s JOIN `%s` AS %s ON %s.id = %s.`%s` WHERE %s.`%s` = `x`.id AND %s)",
				pivotTable,
				pivotAlias,
				relatedTable,
				alias,
				alias,
				pivotAlias,
				relatedPivotCol,
				pivotAlias,
				listedPivotCol,
				strings.Join(relatedPreds, " AND "),
			))
		case relatedHasFK:
			preds = append(preds, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM `%s` AS %s WHERE %s.`%s` = `x`.id AND %s)",
				relatedTable,
				alias,
				alias,
				relatedFK,
				strings.Join(relatedPreds, " AND "),
			))
		default:
			return nil, fmt.Errorf("could not resolve SQL relation path for %s.%s", param.Model.Name, relationKey)
		}
	}
	return preds, nil
}

func CommonDocTransformation(model *models.ModelType, local string, result map[string]interface{}, classification *FieldClassification) (*types.DefaultDocumentStructure, error) {

	doc := types.DefaultDocumentStructure{
		Type: model.Name,
		Meta: &types.MetaField{},
	}

	if val, ok := result["id"].(string); ok {
		doc.ID = string(val)
		doc.Key = doc.ID
	} else {
		return nil, errors.New("id is required for any document to fetch")
	}

	data := map[string]interface{}{}

	for k, v := range result {
		switch k {
		case "doc_id":
			continue
		case "sys_key":
			continue
		case "id":
			continue
		case "sys_status":
			doc.Meta.Status = sqlMetaString(v)
		case "sys_created_at":
			s, err := formatSQLMetaTimestamp(v)
			if err != nil {
				return nil, err
			}
			doc.Meta.CreatedAt = s
		case "sys_updated_at":
			s, err := formatSQLMetaTimestamp(v)
			if err != nil {
				return nil, err
			}
			doc.Meta.UpdatedAt = s
		case "sys_updated_by":
			if id := sqlMetaString(v); id != "" {
				doc.Meta.LastModifiedBy = &types.SystemUser{
					ID: id,
				}
			}
		case "sys_created_by":
			if id := sqlMetaString(v); id != "" {
				doc.Meta.CreatedBy = &types.SystemUser{
					ID: id,
				}
			}
		default:
			if utility.ArrayContains(classification.MultilineFields, k) {
				var html string
				if val, ok := v.(string); ok {
					html = val
				}
				// Use the new markdown processor
				processed := utility.ProcessMultilineField(map[string]interface{}{
					"html": html,
				})
				data[k] = processed
			} else if utility.ArrayContains(classification.PictureField, k) {
				if val, ok := v.(string); ok {
					var pic map[string]interface{}
					err := json.Unmarshal([]byte(val), &pic)
					if err != nil {
						return nil, err
					}
					data[k] = pic
				}
			} else if utility.ArrayContains(classification.GalleryField, k) {
				if val, ok := v.(string); ok {
					var gallery []map[string]interface{}
					err := json.Unmarshal([]byte(val), &gallery)
					if err != nil {
						return nil, err
					}
					data[k] = gallery
				}
			} else if utility.ArrayContains(classification.ListFields, k) {
				if val, ok := v.(string); ok {
					var lists []interface{}
					err := json.Unmarshal([]byte(val), &lists)
					if err != nil {
						return nil, err
					}
					data[k] = lists
				}
			} else if subfields, ok := classification.RepeatedFields[k]; ok && len(classification.RepeatedFields) > 0 {
				var repeated []map[string]interface{}
				if val, ok := v.(string); ok {
					err := json.Unmarshal([]byte(val), &repeated)
					if err != nil {
						return nil, err
					}
				}
				if local == "en" {
					data[k] = repeated
				} else {
					for _, subItem := range repeated {
						for _, f := range subfields {
							if f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) {
								if localContentFound, ok := subItem[fmt.Sprintf(`%s_%s`, f.Identifier, local)]; ok {
									subItem[f.Identifier] = localContentFound
								}
								break
							}
						}
					}
					data[k] = repeated
				}
			} else if utility.ArrayContains(classification.ObjectField, k) {
				if val, ok := v.(string); ok {
					var object map[string]interface{}
					err := json.Unmarshal([]byte(val), &object)
					if err != nil {
						return nil, err
					}
					data[k] = object
				}
			} else if utility.ArrayContains(classification.BooleanFields, k) {
				if val, ok := v.(int64); ok {
					data[k] = val == 1
				}
			} else if utility.ArrayContains(classification.DateFields, k) {
				s, err := formatSQLMetaTimestamp(v)
				if err != nil {
					return nil, err
				}
				if s != "" {
					data[k] = s
				}
			} else {
				data[k] = v
			}
		}
	}
	doc.Data = data
	return &doc, nil
}

func MediaDocTransformation(docType string, result map[string]interface{}) (*models.FileDetails, error) {

	doc := models.FileDetails{
		Type: docType,
	}

	if val, ok := result["id"].([]byte); ok {
		doc.ID = string(val)
		doc.XKey = doc.ID
	} else {
		return nil, nil
	}

	if s, err := formatSQLMetaTimestamp(result["created_at"]); err != nil {
		return nil, err
	} else if s != "" {
		doc.CreatedAt = s
	}

	if val, ok := result["model"].(string); ok {
		if doc.UploadParam == nil {
			doc.UploadParam = &models.UploadParams{}
		}
		doc.UploadParam.ModelName = val
	}

	if val, ok := result["s3_key"].(string); ok {
		doc.S3Key = val
	}

	if val, ok := result["media_type"].(string); ok {
		doc.ContentType = val
	}

	if val, ok := result["file_extension"].(string); ok {
		doc.FileExtension = val
	}

	if val, ok := result["file_name"].(string); ok {
		doc.FileName = val
	}

	if val, ok := result["size"].(int32); ok {
		doc.Size = int64(val)
	}

	if val, ok := result["url"].(string); ok {
		doc.URL = val
	}

	return &doc, nil
}

func RootConnectionResolverQueryBuilder(cfg *models.Config, param *models.CommonSystemParams) (string, error) {

	projectId := param.ProjectID
	_args := param.ResolveParams.Args

	filters, err := ConditionBuilder("x.data", _args, param.Model, nil)
	if err != nil {
		return "", err
	}
	if err := mergeQueryFilterHookAQL(cfg, param, filters, "x"); err != nil {
		return "", err
	}

	var mergedFilter []string
	for condition, _ := range filters {
		mergedFilter = append(mergedFilter, strings.Join(filters[condition], condition))
	}

	model := param.Model.Name

	var queries []string
	queries = append(queries, fmt.Sprintf("FOR x in `p_%s`", projectId))
	if len(filters) > 0 {
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s' AND %s`, model, strings.Join(mergedFilter, " AND "))) // #todo need fix
	} else {
		queries = append(queries, fmt.Sprintf(`Filter x.type == '%s'`, model))
	}
	queries = append(queries, fmt.Sprintf(`COLLECT WITH COUNT INTO total`))
	queries = append(queries, fmt.Sprintf(`return total`))

	return strings.Join(queries, " "), nil
}

func RootResolverQueryBuilder(cfg *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {
	return rootResolverQueryBuilder(cfg, param, returnCount, nil, nil)
}

// RootResolverAggregateQueryBuilder builds SELECT SUM(...) … WHERE with the same connection,
// relation, where, role, and tenant filters as list/count (Arango COLLECT AGGREGATE parity).
func RootResolverAggregateQueryBuilder(cfg *models.Config, param *models.CommonSystemParams, selectExprs []string) (string, []interface{}, error) {
	if len(selectExprs) == 0 {
		return "", nil, errors.New("aggregate select expressions required")
	}
	return rootResolverQueryBuilder(cfg, param, true, selectExprs, nil)
}

// RootResolverGroupByQueryBuilder adds GROUP BY to the aggregate query so one row is
// returned per distinct key combination (Arango COLLECT … AGGREGATE parity).
func RootResolverGroupByQueryBuilder(cfg *models.Config, param *models.CommonSystemParams, selectExprs []string, groupByCols []string) (string, []interface{}, error) {
	if len(selectExprs) == 0 {
		return "", nil, errors.New("aggregate select expressions required")
	}
	if len(groupByCols) == 0 {
		return "", nil, errors.New("group by columns required")
	}
	return rootResolverQueryBuilder(cfg, param, true, selectExprs, groupByCols)
}

func rootResolverQueryBuilder(cfg *models.Config, param *models.CommonSystemParams, returnCount bool, selectOverride []string, groupByCols []string) (string, []interface{}, error) {

	var modelName string
	if param.Model == nil {
		modelName = utility.PhysicalSQLTableName(param.ResolveParams.Info.FieldName)
	} else {
		modelName = utility.PhysicalSQLTableName(param.Model.Name)
	}

	var leftJoins []string

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	var returnType []string
	if len(selectOverride) > 0 {
		returnType = selectOverride
	} else {
		returnType = SelectBuilder("y", local, param.Model, returnCount)
	}

	var connection map[string]interface{}
	if val, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok {
		connection = val
	}

	intersect := false
	if param.ResolveParams != nil && param.ResolveParams.Args != nil {
		intersect = graphqlArgBool(param.ResolveParams.Args, "intersect")
	}

	var connPreds []string
	var connArgs []interface{}

	if len(connection) > 0 {
		var relationType string
		if val, ok := connection["relation_type"].(string); ok && val != "" {
			relationType = val
		}

		var connectionType string
		if val, ok := connection["connection_type"].(string); ok && val != "" {
			connectionType = val
		} else {
			return "", nil, errors.New("connection type is required if passing connection object")
		}

		var fromModel string
		var toModel string
		knownAs, _ := connection["known_as"].(string)
		switch connectionType {
		case "forward":
			fromModel = connection["to_model"].(string)
			toModel = connection["model"].(string)
		case "backward":
			fromModel = connection["model"].(string)
			toModel = connection["to_model"].(string)
		default:
			return "", nil, errors.New("invalid connection type")
		}

		switch relationType {
		case "has_many":
			anchorID, okAnchor := sqlConnectionAnchorID(connection)
			if !okAnchor {
				return "", nil, errors.New("connection _id is required when filtering by relation")
			}
			// Console sends relation_type=has_many for "parent has many children"; SQL may use either a
			// pivot (true M:N) or a FK on the child table (e.g. Work has_one Person → person_id on work).
			usePivot := len(param.ProjectSchemaModels) == 0 || sqlConnectionIsTrueManyToMany(param.ProjectSchemaModels, fromModel, toModel)
			if usePivot {
				pivotTable := relationPivotTableNameParts(fromModel, toModel, knownAs)
				// Pivot columns must follow the *listed* model (x) and *anchor* document (connection._id),
				// not connection fromModel/toModel order (backward vs forward swaps which side is listed).
				if param.Model == nil {
					return "", nil, errors.New("model is required for pivot relation filter")
				}
				listedPivotCol := utility.PhysicalSQLTableName(param.Model.Name) + "_id"
				anchorModelName := sqlConnectionAnchorModelName(connectionType, fromModel, toModel)
				if anchorModelName == "" {
					return "", nil, errors.New("invalid connection type")
				}
				anchorPivotCol := utility.PhysicalSQLTableName(anchorModelName) + "_id"
				if intersect {
					// Same as Arango: listed rows whose id is NOT IN pivot ids linked to the anchor.
					if !returnCount {
						returnType = SelectBuilder("y", local, param.Model, false)
					}
					connPreds = append(connPreds, fmt.Sprintf("`x`.`id` NOT IN (SELECT `p`.`%s` FROM `%s` AS `p` WHERE `p`.`%s` = ?)", listedPivotCol, pivotTable, anchorPivotCol))
					connArgs = append(connArgs, anchorID)
				} else {
					leftJoins = append(leftJoins, fmt.Sprintf(`left join `+"`%s`"+` as z on z.%s = x.id`, pivotTable, listedPivotCol))
					// Keep default returnType (full `x` row + meta): pivot only filters; overwriting
					// returnType would omit `id` and break CommonDocTransformation.
					connPreds = append(connPreds, fmt.Sprintf("`z`.`%s` = ?", anchorPivotCol))
					connArgs = append(connArgs, anchorID)
				}
			} else {
				xfk := relationFKColumnNameParts(fromModel, knownAs)
				if intersect {
					connPreds = append(connPreds, fmt.Sprintf("(`x`.`%s` IS NULL OR `x`.`%s` != ?)", xfk, xfk))
					connArgs = append(connArgs, anchorID)
				} else {
					connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", xfk))
					connArgs = append(connArgs, anchorID)
				}
			}
		case "has_one":
			anchorID, okAnchor := sqlConnectionAnchorID(connection)
			if !okAnchor {
				return "", nil, errors.New("connection _id is required when filtering by relation")
			}
			anchorM := sqlConnectionAnchorModelName(connectionType, fromModel, toModel)
			if anchorM == "" {
				return "", nil, errors.New("invalid connection type")
			}
			listedMT := param.Model
			if listedMT == nil {
				return "", nil, errors.New("model is required for connection filter")
			}
			if len(param.ProjectSchemaModels) > 0 {
				anchorMT := findSchemaModel(param.ProjectSchemaModels, anchorM)
				if col, ok := fkPhysicalColumnOnModelToTarget(listedMT, anchorM, knownAs); ok {
					if intersect {
						connPreds = append(connPreds, fmt.Sprintf("(`x`.`%s` IS NULL OR `x`.`%s` != ?)", col, col))
						connArgs = append(connArgs, anchorID)
					} else {
						connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", col))
						connArgs = append(connArgs, anchorID)
					}
				} else if anchorMT != nil {
					if col, ok := fkPhysicalColumnOnModelToTarget(anchorMT, listedMT.Name, knownAs); ok {
						anchTbl := utility.PhysicalSQLTableName(anchorM)
						if intersect {
							connPreds = append(connPreds, fmt.Sprintf("NOT EXISTS (SELECT 1 FROM `%s` AS anch WHERE anch.id = ? AND anch.`%s` = `x`.`id`)", anchTbl, col))
							connArgs = append(connArgs, anchorID)
						} else {
							connPreds = append(connPreds, fmt.Sprintf("EXISTS (SELECT 1 FROM `%s` AS anch WHERE anch.id = ? AND anch.`%s` = `x`.`id`)", anchTbl, col))
							connArgs = append(connArgs, anchorID)
						}
					} else {
						return "", nil, fmt.Errorf("could not resolve SQL FK for has_one connection (listed=%s anchor=%s)", listedMT.Name, anchorM)
					}
				} else {
					return "", nil, fmt.Errorf("unknown anchor model %q for has_one connection", anchorM)
				}
			} else {
				col := relationFKColumnNameParts(anchorM, knownAs)
				if intersect {
					connPreds = append(connPreds, fmt.Sprintf("(`x`.`%s` IS NULL OR `x`.`%s` != ?)", col, col))
					connArgs = append(connArgs, anchorID)
				} else {
					connPreds = append(connPreds, fmt.Sprintf("`x`.`%s` = ?", col))
					connArgs = append(connArgs, anchorID)
				}
			}
		}
	}

	limit, offset := LimitBuilder(param.ResolveParams)

	tableName := utility.PhysicalSQLTableName(param.Model.Name)

	ownFilter := false
	if !utility.RoleBypassesDataACL(param.Role) {
		if permission := utility.EffectivePermission(param.Role, modelName); permission != nil && permission.Read == "own" {
			ownFilter = true
		}
	}

	// Meta join is needed for list projection and for own ACL (list + count).
	if (!returnCount || ownFilter) && (param.Model == nil || !models.ModelIsProjectAuthUserModel(param.Model)) {
		leftJoins = append(leftJoins, `left join meta as y on y.doc_id = x.id`)
	}

	var queries []string
	queries = append(queries, fmt.Sprintf("SELECT %s FROM `%s` as x %s",
		strings.Join(returnType, ", "),
		tableName,
		strings.Join(leftJoins, "\n"),
	))

	var sqlArgs []interface{}
	filters, err := ConditionBuilder("x", param.ResolveParams.Args, param.Model, &sqlArgs)
	if err != nil {
		return "", nil, err
	}
	relationPreds, err := publicRelationFilterPredicates(param, &sqlArgs)
	if err != nil {
		return "", nil, err
	}
	if len(relationPreds) > 0 {
		filters["AND"] = append(filters["AND"], relationPreds...)
	}

	// filter based on roles (own applies to list and count)
	if ownFilter {
		filters["AND"] = append(filters["AND"], `y.created_by = ?`)
		sqlArgs = append(sqlArgs, param.UserID)
	}
	if err := mergeQueryFilterHookSQL(cfg, param, filters, "x", &sqlArgs); err != nil {
		return "", nil, err
	}
	if len(param.DocumentIDs) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(param.DocumentIDs)), ",")
		filters["AND"] = append(filters["AND"], fmt.Sprintf(`x.id IN (%s)`, ph))
		for _, id := range param.DocumentIDs {
			sqlArgs = append(sqlArgs, id)
		}
	}
	for i := range connPreds {
		filters["AND"] = append(filters["AND"], connPreds[i])
		sqlArgs = append(sqlArgs, connArgs[i])
	}

	mergedFilter := mergeGroupedSQLPredicates(filters)

	if len(mergedFilter) > 0 {
		queries = append(queries, fmt.Sprintf(`WHERE %s`, strings.Join(mergedFilter, " AND ")))
	}

	if len(groupByCols) > 0 {
		queries = append(queries, fmt.Sprintf(`GROUP BY %s`, strings.Join(groupByCols, ", ")))
	}

	// default sort (or client sort arg)
	if !returnCount { // limit & Order if not counting
		queries = append(queries, sqlquery.RootListOrderByFromArgs(param))
		queries = append(queries, fmt.Sprintf(`LIMIT %d OFFSET %d`, limit, offset))
	}

	query := strings.Join(queries, " ")

	return query, sqlArgs, nil
}

func BuildCombinedRelationQuery(cfg *models.Config, relationType string, parentModel string, arg *models.CommonSystemParams) (string, []interface{}, *string, error) {

	var local string
	if val, ok := arg.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	var filterArgs []interface{}
	filters, err := ConditionBuilder("x", arg.ResolveParams.Args, arg.Model, &filterArgs)
	if err != nil {
		return "", nil, nil, err
	}
	if err := mergeQueryFilterHookSQL(cfg, arg, filters, "x", &filterArgs); err != nil {
		return "", nil, nil, err
	}

	mergedFilter := mergeGroupedSQLPredicates(filters)

	knownAsFilter := strings.TrimSpace(arg.KnownAs)
	if knownAsFilter == "" && arg.ResolveParams != nil && arg.ResolveParams.Args != nil {
		if s, ok := arg.ResolveParams.Args["known_as"].(string); ok {
			knownAsFilter = strings.TrimSpace(s)
		}
	}

	var relationshipDirection string
	for _, m := range arg.Model.Connections {
		if m == nil || !relationEdgeMatchesParentModel(m, parentModel) {
			continue
		}
		if knownAsFilter != "" && strings.TrimSpace(m.KnownAs) != "" && strings.TrimSpace(m.KnownAs) != knownAsFilter {
			continue
		}
		if m.Type == "backward" {
			relationshipDirection = "to"
		} else if m.Type == "forward" {
			relationshipDirection = "from"
		}
		if relationshipDirection != "" {
			break
		}
	}

	if relationshipDirection == "" {
		return "", nil, nil, errors.New("could not decide form/to relations")
	}

	var relationInput map[string]interface{}
	if len(arg.ResolveParams.Args) > 0 {
		relationInput = arg.ResolveParams.Args
	} else {
		relationInput = map[string]interface{}{
			"from_model":    parentModel,
			"to_model":      arg.Model.Name,
			"relation_type": relationType,
			"known_as":      "",
		}
	}

	keys := arg.DocumentIDs
	if len(keys) == 0 {
		return "", nil, nil, errors.New("BuildCombinedRelationQuery: document ids required")
	}
	keyPH := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	keyArgs := make([]interface{}, len(keys))
	for i, k := range keys {
		keyArgs[i] = k
	}

	selectThing := SelectBuilder("z", local, arg.Model, arg.OnlyReturnCount)
	selectList := strings.Join(selectThing, ", ")
	metaJoin := sqlquery.RelationMetaJoinFragment(arg.Model)

	relationTo := relationInput["to_model"].(string)
	tableName := utility.PhysicalSQLTableName(relationTo)
	parentCol := utility.PhysicalSQLTableName(parentModel)
	fromModelStr := relationInput["from_model"].(string)
	fromCol := utility.PhysicalSQLTableName(fromModelStr)
	relationToCol := utility.PhysicalSQLTableName(relationTo)
	knownAs, _ := relationInput["known_as"].(string)

	var query string
	var whereCondition string
	var pivotTable string
	switch relationInput["relation_type"] {
	case "has_many":
		var manyToManyRelation bool
		for _, c := range arg.Model.Connections {
			if utility.ModelIDMatchesGraphQLField(c.Model, parentModel) {
				if c.Relation == "has_many" {
					manyToManyRelation = true
					break
				}
			}
		}

		var keyField string
		if manyToManyRelation {

			if len(mergedFilter) > 0 {
				whereCondition = fmt.Sprintf(`y.%s_id IN (%s) AND %s`, parentCol, keyPH, strings.Join(mergedFilter, " AND "))
			} else {
				whereCondition = fmt.Sprintf(`y.%s_id IN (%s)`, parentCol, keyPH)
			}

			keyField = fmt.Sprintf(`y.%s_id`, parentCol)

			// Pivot physical table name is canonical (lexicographic pair); do not swap by relationshipDirection.
			pivotTable = relationPivotTableNameParts(fromModelStr, relationTo, knownAs)
			// Alias must match RelationshipDataLoader* (reads sys_key to bucket rows per parent id).
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.id = y.%s_id%s
				WHERE %s`, keyField, selectList, pivotTable, tableName, relationToCol, metaJoin, whereCondition)
		} else {

			if len(mergedFilter) > 0 {
				whereCondition = fmt.Sprintf(`y.id IN (%s) AND %s`, keyPH, strings.Join(mergedFilter, " AND "))
			} else {
				whereCondition = fmt.Sprintf(`y.id IN (%s)`, keyPH)
			}

			keyField = fmt.Sprintf(`y.id`)

			switch relationshipDirection {
			case "to":
				pivotTable = fromCol
			case "from":
				pivotTable = relationToCol
			}
			hasManyFK := relationFKColumnNameParts(fromModelStr, knownAs)
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.%s = y.id%s
				WHERE %s`, keyField, selectList, pivotTable, tableName, hasManyFK, metaJoin, whereCondition)
		}
	case "has_one":
		if len(mergedFilter) > 0 {
			whereCondition = fmt.Sprintf(`y.id IN (%s) AND %s`, keyPH, strings.Join(mergedFilter, " AND "))
		} else {
			whereCondition = fmt.Sprintf(`y.id IN (%s)`, keyPH)
		}

		keyField := `y.id`

		var isOneToOne bool
		for _, c := range arg.Model.Connections {
			if utility.ModelIDMatchesGraphQLField(c.Model, parentModel) && c.Relation == "has_one" {
				isOneToOne = true
				break
			}
		}

		switch relationshipDirection {
		case "to":
			pivotTable = fromCol
			if isOneToOne {
				hasOneFK := relationFKColumnNameParts(fromModelStr, knownAs)
				query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
					LEFT JOIN `+"`%s`"+` AS x ON x.%s = y.id%s
					WHERE %s`, keyField, selectList, pivotTable, tableName, hasOneFK, metaJoin, whereCondition)
			} else {
				hasOneFK := relationFKColumnNameParts(relationTo, knownAs)
				query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
					LEFT JOIN `+"`%s`"+` AS x ON x.id = y.%s%s
					WHERE %s`, keyField, selectList, pivotTable, tableName, hasOneFK, metaJoin, whereCondition)
			}
		case "from":
			pivotTable = fromCol
			hasOneFK := relationFKColumnNameParts(relationTo, knownAs)
			query = fmt.Sprintf(`SELECT %s AS sys_key, %s FROM `+"`%s`"+` AS y 
				LEFT JOIN `+"`%s`"+` AS x ON x.id = y.%s%s
				WHERE %s`, keyField, selectList, pivotTable, tableName, hasOneFK, metaJoin, whereCondition)
		}
	default:
		return "", nil, nil, fmt.Errorf("unsupported relation_type %v", relationInput["relation_type"])
	}

	rt := relationInput["relation_type"].(string)
	outArgs := append(keyArgs, filterArgs...)
	return query, outArgs, &rt, nil
}

func RootResolverMediaQueryBuilder(param *graphql.ResolveParams) (string, error) {

	limit, offset := LimitBuilder(param)
	var queries []string
	queries = append(queries, `SELECT * FROM media AS x`)

	if val, ok := param.Args["model"].(string); ok {
		if val != "" {
			queries = append(queries, fmt.Sprintf(`WHERE x.model = '%s'`, val))
		}
	} else if val, ok := param.Args["search"]; ok {
		queries = append(queries, fmt.Sprintf(`WHERE x.file_name LIKE '%%%s%%'`, val))
	}

	// default sort
	queries = append(queries, `ORDER BY x.created_at DESC`)
	queries = append(queries, fmt.Sprintf(`LIMIT %d OFFSET %d`, limit, offset))

	return strings.Join(queries, " "), nil
}

func BuildCombinedMetaQuery(keys dataloader.Keys, param *QueryBuilderParam) ([]byte, error) {

	queries := make(map[string]string)
	for _, key := range keys {
		meta := key.(*models.ResolverKey).GetMeta()
		metaUserIDs := []string{meta.CreatedBy.ID, meta.LastModifiedBy.ID}
		queries[key.String()] = fmt.Sprintf(`(FOR u IN users FILTER u._key in ['%s'] return u)`, strings.Join(metaUserIDs, `','`))
	}

	query, err := json.Marshal(queries)
	if err != nil {
		return nil, err
	}

	first := bytes.ReplaceAll(query, []byte(`:"(`), []byte(`:(`))
	query = bytes.ReplaceAll(first, []byte(`u)"`), []byte(`u)`))

	return bytes.Join([][]byte{[]byte(`return`), query}, []byte(" ")), nil
}
