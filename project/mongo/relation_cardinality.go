package mongo

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/relationmigrate"
)

func (m *MongoDriver) PreviewRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Preview, error) {
	if req == nil {
		req = &relationmigrate.Request{}
	}
	preview := relationmigrate.MetadataOnlyPreview("mongodb", *req)
	return &preview, nil
}

func (m *MongoDriver) MigrateRelationCardinality(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return &relationmigrate.Receipt{ScopeKey: "mongodb", Status: "passed", Idempotent: true}, nil
}

func (m *MongoDriver) VerifyRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return &relationmigrate.Receipt{ScopeKey: "mongodb", Status: "passed"}, nil
}

// EnsureRelationArtifactsFromSchema is a no-op. Mongo relation storage is the edge
// collection created by AddRelationFields; there is no separate column or pivot pass.
func (m *MongoDriver) EnsureRelationArtifactsFromSchema(ctx context.Context, modelsList []*models.ModelType) error {
	return nil
}
