package sqlquery

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
)

func effectiveCfg(cfg *models.Config, param *models.CommonSystemParams) *models.Config {
	if cfg != nil {
		return cfg
	}
	if param != nil {
		return param.RuntimeConfig
	}
	return nil
}

func hookCtx(param *models.CommonSystemParams) context.Context {
	if param != nil && param.ResolveParams != nil && param.ResolveParams.Context != nil {
		return param.ResolveParams.Context
	}
	return context.Background()
}

// MergeQueryFilterHookSQL appends hook-produced equality predicates to filters["AND"] for raw SQL.
func MergeQueryFilterHookSQL(cfg *models.Config, param *models.CommonSystemParams, filters map[string][]string, rowAlias string, sqlArgs *[]interface{}) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return nil
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		varName := strings.TrimSpace(f.Variable)
		if varName == "" {
			varName = rowAlias
		}
		if varName != rowAlias {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		if sqlArgs != nil {
			if err := sqlEqualityClausePlaceholder(rowAlias, key, f.Value, filters, sqlArgs); err != nil {
				return err
			}
			continue
		}
		clause, err := sqlEqualityClause(rowAlias, key, f.Value)
		if err != nil {
			return err
		}
		filters["AND"] = append(filters["AND"], clause)
	}
	return nil
}

func sqlEqualityClausePlaceholder(rowAlias, key string, value interface{}, filters map[string][]string, sqlArgs *[]interface{}) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	for _, r := range key {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("query filter hook: invalid key %q", key)
		}
	}
	filters["AND"] = append(filters["AND"], fmt.Sprintf("`%s`.`%s` = ?", rowAlias, key))
	switch v := value.(type) {
	case string:
		*sqlArgs = append(*sqlArgs, v)
	case int:
		*sqlArgs = append(*sqlArgs, v)
	case int32:
		*sqlArgs = append(*sqlArgs, int64(v))
	case int64:
		*sqlArgs = append(*sqlArgs, v)
	case float32:
		*sqlArgs = append(*sqlArgs, float64(v))
	case float64:
		*sqlArgs = append(*sqlArgs, v)
	case bool:
		*sqlArgs = append(*sqlArgs, v)
	default:
		return fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
	return nil
}

func sqlEqualityClause(alias, key string, value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		esc := strings.ReplaceAll(v, "'", "''")
		return fmt.Sprintf("`%s`.%s = '%s'", alias, key, esc), nil
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("`%s`.%s = %v", alias, key, v), nil
	default:
		return "", fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
}

func mergeQueryFilterHookAQL(cfg *models.Config, param *models.CommonSystemParams, filters map[string][]string, varName string) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return nil
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		vn := strings.TrimSpace(f.Variable)
		if vn == "" {
			vn = varName
		}
		if vn != varName {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" || cond == "=" {
		} else {
			continue
		}
		clause, err := aqlEqualityClause(varName, key, f.Value)
		if err != nil {
			return err
		}
		filters["AND"] = append(filters["AND"], clause)
	}
	return nil
}

func aqlEqualityClause(varName, key string, value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		esc := strings.ReplaceAll(v, "'", "\\'")
		return fmt.Sprintf("%s.%s == '%s'", varName, key, esc), nil
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%s.%s == %v", varName, key, v), nil
	default:
		return "", fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
}
