package sqlcommon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// NormalizeAllProjectPlanSystemFlags walks every project row and persists:
//   - free → system_generated=true (required default)
//   - every other plan → system_generated=false (operator-owned, deletable)
//
// Idempotent; safe on every RunMigration. Does not add/remove product tiers —
// that stays in Console / MCP / upsert-*-plans scripts.
func NormalizeAllProjectPlanSystemFlags(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for project plan system_generated normalize")
	}
	var projects []*models.Project
	if err := db.NewSelect().
		Model(&projects).
		Column("id", "plans", "plan_seed_omissions", "updated_at").
		Scan(ctx); err != nil {
		return fmt.Errorf("list projects for plan flag normalize: %w", err)
	}
	for _, project := range projects {
		if project == nil || project.ID == "" {
			continue
		}
		if !models.EnsureProjectPlansSeeds(project) {
			continue
		}
		project.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if _, err := db.NewUpdate().
			Model(project).
			Column("plans", "plan_seed_omissions", "updated_at").
			Where("id = ?", project.ID).
			Exec(ctx); err != nil {
			return fmt.Errorf("persist plan flags for project %s: %w", project.ID, err)
		}
	}
	return nil
}
