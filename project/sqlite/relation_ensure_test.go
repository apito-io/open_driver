package sqlite

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestFkPhysicalColumnOnModelToTarget(t *testing.T) {
	work := &models.ModelType{
		Name: "work",
		Fields: []*models.FieldInfo{
			{Identifier: "system_person_id", SystemGenerated: true},
		},
	}
	col, ok := fkPhysicalColumnOnModelToTarget(work, "person", "")
	require.True(t, ok)
	require.Equal(t, "person_id", col)

	_, ok = fkPhysicalColumnOnModelToTarget(work, "tenant", "")
	require.False(t, ok)
}

func TestFkPhysicalColumnOnModelToTargetFromConnection(t *testing.T) {
	book := &models.ModelType{
		Name: "book",
		Connections: []*models.ConnectionType{
			{Model: "users", Relation: "has_one"},
		},
	}
	col, ok := fkPhysicalColumnOnModelToTarget(book, "users", "")
	require.True(t, ok)
	require.Equal(t, "users_id", col)
}

func TestSqlConnectionAnchorModelName(t *testing.T) {
	require.Equal(t, "person", sqlConnectionAnchorModelName("forward", "person", "work"))
	require.Equal(t, "work", sqlConnectionAnchorModelName("backward", "person", "work"))
}
