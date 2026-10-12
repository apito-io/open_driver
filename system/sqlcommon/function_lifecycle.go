package sqlcommon

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// SQLFunctionLifecycleStore persists function_* lifecycle tables via bun.
type SQLFunctionLifecycleStore struct {
	orm *bun.DB
}

// NewSQLFunctionLifecycleStore wraps a bun DB.
func NewSQLFunctionLifecycleStore(orm *bun.DB) *SQLFunctionLifecycleStore {
	return &SQLFunctionLifecycleStore{orm: orm}
}

// FunctionLifecycleStore returns the store for sqlcommon.Base embedders.
func (b *Base) FunctionLifecycleStore() interfaces.FunctionLifecycleStore {
	if b == nil || b.ORM == nil {
		return nil
	}
	return NewSQLFunctionLifecycleStore(b.ORM)
}

func (s *SQLFunctionLifecycleStore) SaveRevision(ctx context.Context, rev *models.FunctionRevision) error {
	if s == nil || s.orm == nil || rev == nil || rev.ID == "" {
		return fmt.Errorf("invalid revision")
	}
	_, err := s.orm.NewInsert().Model(rev).Exec(ctx)
	if err != nil {
		if !IsSQLUniqueViolation(err) {
			return err
		}
		_, err = s.orm.NewUpdate().Model(rev).WherePK().Exec(ctx)
	}
	return err
}

func (s *SQLFunctionLifecycleStore) GetRevision(ctx context.Context, projectID, revisionID string) (*models.FunctionRevision, error) {
	if s == nil || s.orm == nil || revisionID == "" {
		return nil, fmt.Errorf("revision not found")
	}
	rev := new(models.FunctionRevision)
	q := s.orm.NewSelect().Model(rev).Where("id = ?", revisionID)
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	return rev, nil
}

func (s *SQLFunctionLifecycleStore) ListRevisions(ctx context.Context, projectID, functionName string, limit int) ([]*models.FunctionRevision, error) {
	if s == nil || s.orm == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []*models.FunctionRevision
	err := s.orm.NewSelect().
		Model(&rows).
		Where("project_id = ? AND name = ?", projectID, functionName).
		OrderExpr("revision DESC, created_at DESC").
		Limit(limit).
		Scan(ctx)
	return rows, err
}

func (s *SQLFunctionLifecycleStore) SaveBuild(ctx context.Context, build *models.FunctionBuild) error {
	if s == nil || s.orm == nil || build == nil || build.ID == "" {
		return fmt.Errorf("invalid build")
	}
	_, err := s.orm.NewInsert().Model(build).Exec(ctx)
	return err
}

func (s *SQLFunctionLifecycleStore) SaveDeployment(ctx context.Context, dep *models.FunctionDeployment) error {
	if s == nil || s.orm == nil || dep == nil || dep.ID == "" {
		return fmt.Errorf("invalid deployment")
	}
	_, err := s.orm.NewInsert().Model(dep).Exec(ctx)
	return err
}

func (s *SQLFunctionLifecycleStore) ListDeployments(ctx context.Context, projectID, functionName string, limit int) ([]*models.FunctionDeployment, error) {
	if s == nil || s.orm == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	var rows []*models.FunctionDeployment
	err := s.orm.NewSelect().
		Model(&rows).
		Where("project_id = ? AND name = ?", projectID, functionName).
		OrderExpr("created_at DESC").
		Limit(limit).
		Scan(ctx)
	return rows, err
}

func (s *SQLFunctionLifecycleStore) MarkDeploymentsSuperseded(ctx context.Context, projectID, functionName, exceptID string) error {
	if s == nil || s.orm == nil {
		return nil
	}
	q := s.orm.NewUpdate().
		Model((*models.FunctionDeployment)(nil)).
		Set("status = ?", "superseded").
		Where("project_id = ? AND name = ? AND status = ?", projectID, functionName, "active")
	if exceptID != "" {
		q = q.Where("id <> ?", exceptID)
	}
	_, err := q.Exec(ctx)
	return err
}

func (s *SQLFunctionLifecycleStore) SaveInvocation(ctx context.Context, inv *models.FunctionInvocation) error {
	if s == nil || s.orm == nil || inv == nil || inv.ID == "" {
		return fmt.Errorf("invalid invocation")
	}
	_, err := s.orm.NewInsert().Model(inv).Exec(ctx)
	return err
}
