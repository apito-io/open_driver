package mongo

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// mergeQueryFilterBSON merges hook filters into an existing bson.M (AND semantics for equality keys).
func mergeQueryFilterBSON(cfg *models.Config, param *models.CommonSystemParams, filter bson.M) {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil || filter == nil {
		return
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		if vn := strings.TrimSpace(f.Variable); vn != "" {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond != "=" && cond != "==" {
			continue
		}
		filter[key] = f.Value
	}
}

// mergeOwnReadFilterBSON adds an indexed candidate filter for own-scoped
// reads. The decoded document is still checked with mongoOwnReadAllowed so
// conflicting canonical fields and legacy fallback rules cannot be bypassed
// by a broad Mongo $or.
func mergeOwnReadFilterBSON(param *models.CommonSystemParams, filter bson.M) bool {
	if filter == nil {
		return false
	}
	userID, ownRead := utility.OwnReadUserID(param)
	if !ownRead {
		return false
	}
	if userID == "" {
		filter["$expr"] = bson.M{"$eq": bson.A{1, 0}}
		return true
	}

	candidate := bson.M{
		"$or": bson.A{
			bson.M{"id": userID},
			bson.M{"_id": userID},
			bson.M{"data.owner_id": userID},
			bson.M{"data.owner_uid": userID},
			bson.M{"meta.created_by._id": userID},
			bson.M{"meta.created_by.id": userID},
		},
	}
	if len(filter) == 0 {
		for key, value := range candidate {
			filter[key] = value
		}
		return true
	}

	existing := make(bson.M, len(filter))
	for key, value := range filter {
		existing[key] = value
		delete(filter, key)
	}
	filter["$and"] = bson.A{existing, candidate}
	return true
}

func mongoOwnReadAllowed(param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) bool {
	if param == nil {
		return false
	}
	userID, ownRead := utility.OwnReadUserID(param)
	if !ownRead {
		return true
	}
	modelName := ""
	if param.Model != nil {
		modelName = param.Model.Name
	}
	return utility.OwnDocumentBelongsToUser(doc, modelName, userID)
}

func applyMongoOwnReadPagination(
	docs []*types.DefaultDocumentStructure,
	param *models.CommonSystemParams,
) []*types.DefaultDocumentStructure {
	if param == nil || param.ResolveParams == nil {
		return docs
	}
	skip := 0
	if value, ok := param.ResolveParams.Args["skip"].(int); ok && value > 0 {
		skip = value
	}
	if skip >= len(docs) {
		if skip > 0 {
			return []*types.DefaultDocumentStructure{}
		}
		return docs
	}
	if skip > 0 {
		docs = docs[skip:]
	}
	if limit, ok := param.ResolveParams.Args["limit"].(int); ok && limit > 0 && limit < len(docs) {
		return docs[:limit]
	}
	return docs
}

func runDocumentPreInsertHook(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, row map[string]interface{}) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.DocumentPreInsertHook == nil || param == nil || row == nil {
		return nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	return cfg.DocumentPreInsertHook(hctx, param, row)
}

func mergeHookRowIntoDocData(doc *types.DefaultDocumentStructure, row map[string]interface{}) {
	if doc == nil || len(row) == 0 {
		return
	}
	if doc.Data == nil {
		doc.Data = make(map[string]interface{}, len(row))
	}
	for k, v := range row {
		if _, exists := doc.Data[k]; !exists {
			doc.Data[k] = v
		}
	}
}

func runDocumentPreInsertDocHook(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.DocumentPreInsertDocHook == nil || param == nil || doc == nil {
		return nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	return cfg.DocumentPreInsertDocHook(hctx, param, doc)
}
