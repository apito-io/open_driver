package cloudflared1

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/relationmigrate"
)

func (d *Driver) PreviewRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Preview, error) {
	if req == nil {
		req = &relationmigrate.Request{}
	}
	preview := relationmigrate.UnsupportedPreview("cloudflare-d1", *req)
	return &preview, nil
}

func (d *Driver) MigrateRelationCardinality(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return nil, fmt.Errorf("unsupported_driver: cloudflare d1 relation cardinality migration is not implemented")
}

func (d *Driver) VerifyRelationCardinalityMigration(ctx context.Context, req *relationmigrate.Request) (*relationmigrate.Receipt, error) {
	return nil, fmt.Errorf("unsupported_driver: cloudflare d1 relation cardinality migration is not implemented")
}
