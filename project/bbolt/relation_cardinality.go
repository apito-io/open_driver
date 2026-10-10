package bbolt

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/relationmigrate"
)

func (b *BBoltDriver) PreviewRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Preview, error) {
	if req == nil {
		req = &relationmigrate.Request{}
	}
	preview := relationmigrate.MetadataOnlyPreview("bbolt", *req)
	return &preview, nil
}

func (b *BBoltDriver) MigrateRelationCardinality(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return &relationmigrate.Receipt{ScopeKey: "bbolt", Status: "passed", Idempotent: true}, nil
}

func (b *BBoltDriver) VerifyRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return &relationmigrate.Receipt{ScopeKey: "bbolt", Status: "passed"}, nil
}

// EnsureRelationArtifactsFromSchema is a no-op. BBolt relation storage is created by
// AddRelationFields; there is no separate column or pivot pass.
func (b *BBoltDriver) EnsureRelationArtifactsFromSchema(ctx context.Context, modelsList []*models.ModelType) error {
	return nil
}
