package sqlquery

import (
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
)

func TestRootResolverQueryBuilder_CommonModelHidesDeleted(t *testing.T) {
	model := &models.ModelType{
		Name: "catalog_item",
		Ext:  map[string]interface{}{models.ExtKeyIsCommonModel: true},
		Fields: []*models.FieldInfo{
			{Identifier: "name", FieldType: "text"},
		},
	}
	param := &models.CommonSystemParams{
		Model: model,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{},
		},
	}
	for _, returnCount := range []bool{false, true} {
		q, _, err := RootResolverQueryBuilder(nil, param, returnCount)
		require.NoError(t, err)
		require.Contains(t, q, "left join meta as y")
		require.Contains(t, strings.ToLower(q), "y.status")
		require.Contains(t, strings.ToLower(q), "deleted")
	}
}

func TestHideDeletedCommonDocument(t *testing.T) {
	common := &models.ModelType{Name: "catalog_item", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	tenant := &models.ModelType{Name: "tenant_record"}
	deleted := &types.DefaultDocumentStructure{ID: "x", Meta: &types.MetaField{Status: "deleted"}}
	live := &types.DefaultDocumentStructure{ID: "y", Meta: &types.MetaField{Status: "published"}}
	require.Nil(t, HideDeletedCommonDocument(common, deleted))
	require.Equal(t, "y", HideDeletedCommonDocument(common, live).ID)
	require.Equal(t, "x", HideDeletedCommonDocument(tenant, deleted).ID)
}
