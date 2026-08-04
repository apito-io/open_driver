package sqlquery

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
)

func TestRootListOrderByFromArgs_usesSortableFields(t *testing.T) {
	model := &models.ModelType{
		Name: "student",
		Fields: []*models.FieldInfo{
			{Identifier: "roll_no", FieldType: "text"},
			{Identifier: "bio", FieldType: "multiline"},
			{Identifier: "name", FieldType: "text"},
		},
	}
	param := &models.CommonSystemParams{
		Model: model,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"sort": map[string]interface{}{
					"roll_no": "ASC",
					"bio":     "DESC", // not sortable — ignored
				},
			},
		},
	}
	got := RootListOrderByFromArgs(param)
	require.Contains(t, got, "`x`.`roll_no` ASC")
	require.NotContains(t, got, "bio")
	require.Contains(t, got, "`x`.`id` ASC")
}

func TestRootListOrderByFromArgs_fallsBackDefault(t *testing.T) {
	model := &models.ModelType{Name: "student"}
	param := &models.CommonSystemParams{Model: model}
	got := RootListOrderByFromArgs(param)
	require.Equal(t, RootListOrderBySQL(model), got)
}

func TestRootListOrderByFromArgs_normalizesLowercase(t *testing.T) {
	model := &models.ModelType{
		Name: "student",
		Fields: []*models.FieldInfo{
			{Identifier: "roll_no", FieldType: "text"},
		},
	}
	param := &models.CommonSystemParams{
		Model: model,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"sort": map[string]interface{}{"roll_no": "asc"},
			},
		},
	}
	got := RootListOrderByFromArgs(param)
	require.Contains(t, got, "`x`.`roll_no` ASC")
}
