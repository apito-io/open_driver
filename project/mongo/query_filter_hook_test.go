package mongo

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMergeQueryFilterBSON(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "region",
				Condition: "==",
				Value:     "eu-west",
			}}
		},
	}
	f := bson.M{"_id": "1"}
	mergeQueryFilterBSON(cfg, &models.CommonSystemParams{}, f)
	require.Equal(t, "eu-west", f["region"])
}

func TestMergeOwnReadFilterBSONAndDecodedCheck(t *testing.T) {
	param := &models.CommonSystemParams{
		UserID: "user-a",
		Model:  &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "author",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
		},
	}
	filter := bson.M{"tenant_id": "tenant-a"}
	require.True(t, mergeOwnReadFilterBSON(param, filter))
	require.Contains(t, filter, "$and")

	require.True(t, mongoOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a"},
	}))
	require.False(t, mongoOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a", "owner_uid": "user-b"},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "user-a"}},
	}))
	require.False(t, mongoOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "user-b"}},
	}))
}

func TestMergeOwnReadFilterBSONDeniesMissingCaller(t *testing.T) {
	param := &models.CommonSystemParams{
		Model: &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "author",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
		},
	}
	filter := bson.M{}
	require.True(t, mergeOwnReadFilterBSON(param, filter))
	require.Contains(t, filter, "$expr")
	require.False(t, mongoOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a"},
	}))
}

func TestApplyMongoOwnReadPaginationAfterFiltering(t *testing.T) {
	param := &models.CommonSystemParams{
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{"skip": 1, "limit": 1},
		},
	}
	docs := []*types.DefaultDocumentStructure{
		{ID: "first"},
		{ID: "second"},
		{ID: "third"},
	}
	result := applyMongoOwnReadPagination(docs, param)
	require.Len(t, result, 1)
	require.Equal(t, "second", result[0].ID)
}

func TestRunDocumentPreInsertHookMongo(t *testing.T) {
	cfg := &models.Config{
		DocumentPreInsertHook: func(ctx context.Context, p *models.CommonSystemParams, doc map[string]interface{}) error {
			doc["owner_ref"] = "ins"
			return nil
		},
	}
	row := map[string]interface{}{}
	require.NoError(t, runDocumentPreInsertHook(cfg, context.Background(), &models.CommonSystemParams{}, row))
	require.Equal(t, "ins", row["owner_ref"])
}
