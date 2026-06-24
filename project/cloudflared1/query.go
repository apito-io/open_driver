package cloudflared1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"gitlab.com/apito.io/open_driver/project/sqlquery"
)

func (d *Driver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	query, args, err := sqlquery.RootResolverQueryBuilder(d.Conf, param, false)
	if err != nil {
		return nil, err
	}
	rows, err := d.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	local := ""
	if param.ResolveParams != nil {
		if val, ok := param.ResolveParams.Args["local"].(string); ok {
			local = val
		}
	}
	classification := sqlquery.BuildFieldClassification(param.Model.Fields)
	var docs []*types.DefaultDocumentStructure
	for _, row := range rows {
		doc, err := sqlquery.CommonDocTransformation(param.Model, local, row, classification)
		if err != nil {
			return nil, err
		}
		doc.Type = param.Model.Name
		docs = append(docs, doc)
	}
	return docs, nil
}

func (d *Driver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	docs, err := d.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(docs)
}

func (d *Driver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	if n, ok, err := d.tryCountFromRowCountTable(ctx, param); ok {
		return int(n), err
	}
	query, args, err := sqlquery.RootResolverQueryBuilder(d.Conf, param, true)
	if err != nil {
		return nil, err
	}
	row, err := d.DB.QueryRow(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return 0, nil
	}
	for _, v := range row {
		switch t := v.(type) {
		case int64:
			return int(t), nil
		case int:
			return t, nil
		case float64:
			return int(t), nil
		case string:
			n, _ := strconv.Atoi(t)
			return n, nil
		}
	}
	return 0, nil
}

func (d *Driver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	n, err := d.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{"total": n})
}

func (d *Driver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error) {
	n, err := d.CountDocOfProject(ctx, param)
	if err != nil {
		return 0, err
	}
	switch t := n.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	default:
		return 0, fmt.Errorf("unexpected count type %T", n)
	}
}

func (d *Driver) tryCountFromRowCountTable(ctx context.Context, param *models.CommonSystemParams) (int64, bool, error) {
	if !counterTriggersEnabled() {
		return 0, false, nil
	}
	if param == nil || param.Model == nil || param.ResolveParams == nil {
		return 0, false, nil
	}
	w, ok := param.ResolveParams.Args["where"]
	if !ok || w == nil {
		return 0, false, nil
	}
	if m, ok := w.(map[string]interface{}); ok && len(m) > 0 {
		return 0, false, nil
	}
	table := utility.PhysicalSQLTableName(param.Model.Name)
	tenantID := ""
	if d.Conf != nil && d.Conf.QueryFilterHook != nil {
		for _, f := range d.Conf.QueryFilterHook(ctx, param) {
			if f != nil && f.Key == "tenant_id" {
				if s, ok := f.Value.(string); ok {
					tenantID = s
				}
			}
		}
	}
	row, err := d.DB.QueryRow(ctx,
		`SELECT row_count FROM _apito_row_counts WHERE table_name = ? AND tenant_id = ?`,
		table, tenantID,
	)
	if err != nil || row == nil {
		return 0, false, err
	}
	if v, ok := row["row_count"]; ok {
		switch t := v.(type) {
		case int64:
			return t, true, nil
		case float64:
			return int64(t), true, nil
		}
	}
	return 0, false, nil
}

func (d *Driver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	if param == nil || param.ResolveParams == nil {
		return nil, fmt.Errorf("cloudflared1: invalid dataloader params")
	}
	merged := make(map[string]interface{}, len(param.ResolveParams.Args)+8)
	for k, v := range param.ResolveParams.Args {
		merged[k] = v
	}
	if connection != nil {
		if v, ok := connection["to_model"].(string); ok && v != "" {
			merged["from_model"] = utility.PhysicalSQLTableName(v)
		}
		if v, ok := connection["model"].(string); ok && v != "" {
			merged["to_model"] = utility.PhysicalSQLTableName(v)
		}
		for _, k := range []string{"relation_type", "connection_type", "known_as", "_id"} {
			if v, ok := connection[k]; ok {
				merged[k] = v
			}
		}
	}
	orig := param.ResolveParams.Args
	param.ResolveParams.Args = merged
	defer func() { param.ResolveParams.Args = orig }()

	relType, _ := connection["relation_type"].(string)
	query, args, _, err := sqlquery.BuildCombinedRelationQuery(d.Conf, relType, "", param)
	if err != nil {
		return nil, err
	}
	rows, err := d.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	local := ""
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}
	classification := sqlquery.BuildFieldClassification(param.Model.Fields)
	var docs []*types.DefaultDocumentStructure
	for _, row := range rows {
		doc, err := sqlquery.CommonDocTransformation(param.Model, local, row, classification)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (d *Driver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	res, err := d.RelationshipDataLoader(ctx, param, connection)
	if err != nil {
		return nil, err
	}
	return json.Marshal(res)
}
