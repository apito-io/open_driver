package bbolt

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
)

func TestBboltDocPassesQueryHook(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "type",
				Condition: "==",
				Value:     "want",
			}}
		},
	}
	doc := &types.DefaultDocumentStructure{Type: "want"}
	require.True(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
	doc.Type = "other"
	require.False(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
}

func TestBboltDocPassesQueryHook_DataFallback(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "color",
				Condition: "==",
				Value:     "blue",
			}}
		},
	}
	doc := &types.DefaultDocumentStructure{Data: map[string]interface{}{"color": "blue"}}
	require.True(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
}

func TestBboltOwnReadAllowedUsesCanonicalOwnership(t *testing.T) {
	param := &models.CommonSystemParams{
		UserID: "user-a",
		Model:  &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "author",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
		},
	}

	require.True(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a"},
	}))
	require.False(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-b"},
	}))
	require.False(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
	}))
	require.False(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{
			"owner_id":  "user-a",
			"owner_uid": "user-b",
		},
		Meta: &types.MetaField{CreatedBy: &types.SystemUser{ID: "user-a"}},
	}))
}

func TestBboltOwnReadAllowedDeniesEmptyCallerAndKeepsAdminUnfiltered(t *testing.T) {
	param := &models.CommonSystemParams{
		Model: &models.ModelType{Name: "post"},
		Role: &models.Role{
			ID:             "author",
			IsProjectUser:  true,
			APIPermissions: map[string]*models.APIPermission{"post": {Read: "own"}},
		},
	}
	require.False(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a"},
	}))

	param.Role = &models.Role{ID: "admin", IsAdmin: true}
	require.True(t, bboltOwnReadAllowed(param, &types.DefaultDocumentStructure{
		Type: "post",
		Data: map[string]interface{}{"owner_id": "user-a"},
	}))
}

func TestRunDocumentPreInsertHookBbolt(t *testing.T) {
	cfg := &models.Config{
		DocumentPreInsertHook: func(ctx context.Context, p *models.CommonSystemParams, doc map[string]interface{}) error {
			doc["brand"] = "acme"
			return nil
		},
	}
	row := map[string]interface{}{}
	require.NoError(t, runDocumentPreInsertHook(cfg, context.Background(), &models.CommonSystemParams{}, row))
	require.Equal(t, "acme", row["brand"])
}
