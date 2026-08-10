package sqlquery

import (
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
)

func TestRootResolverQueryBuilder_ownFilterAppliesOnCount(t *testing.T) {
	model := &models.ModelType{
		Name: "post",
		Fields: []*models.FieldInfo{
			{Identifier: "title", FieldType: "text"},
		},
	}
	param := &models.CommonSystemParams{
		UserID: "user-42",
		Model:  model,
		Role: &models.Role{
			ID:            "author",
			IsProjectUser: true,
			APIPermissions: map[string]*models.APIPermission{
				"post": {Read: "own", Create: "all", Update: "own", Delete: "none"},
			},
		},
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{},
		},
	}

	for _, tc := range []struct {
		name        string
		returnCount bool
	}{
		{"list", false},
		{"count", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, args, err := RootResolverQueryBuilder(nil, param, tc.returnCount)
			require.NoError(t, err)
			require.Contains(t, q, "y.created_by = ?")
			require.Contains(t, q, "left join meta as y")
			require.Contains(t, args, "user-42")
			if tc.returnCount {
				require.Contains(t, strings.ToLower(q), "count(")
				require.NotContains(t, strings.ToUpper(q), "LIMIT")
			}
		})
	}
}

func TestRootResolverQueryBuilder_ownFilterSkippedForAdmin(t *testing.T) {
	model := &models.ModelType{Name: "post"}
	param := &models.CommonSystemParams{
		UserID: "user-42",
		Model:  model,
		Role:   &models.Role{ID: "admin"},
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{},
		},
	}
	q, args, err := RootResolverQueryBuilder(nil, param, true)
	require.NoError(t, err)
	require.NotContains(t, q, "y.created_by = ?")
	for _, a := range args {
		require.NotEqual(t, "user-42", a)
	}
}
