package sqlcommon

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// SQLUserProjectAccessGranted filters user_projects to accepted (or legacy empty) grants.
const SQLUserProjectAccessGranted = `(invite_status IS NULL OR invite_status = '' OR invite_status = 'accepted')`

// SQLUserProjectAccessGrantedAliased is the same predicate for JOIN alias `up`.
const SQLUserProjectAccessGrantedAliased = `(up.invite_status IS NULL OR up.invite_status = '' OR up.invite_status = 'accepted')`

// MapString reads a TEXT column from a bun map scan (string or []byte).
func MapString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// GetUserProjectGrant returns the raw membership row including pending invites.
func GetUserProjectGrant(ctx context.Context, orm *bun.DB, userID, projectID string) (*models.UserProject, error) {
	if orm == nil {
		return nil, errors.New("orm required")
	}
	row := new(models.UserProject)
	err := orm.NewSelect().
		Model(row).
		Where("user_id = ? AND project_id = ?", userID, projectID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

// LookupGrantsByInviteToken finds all user_projects rows for one invite batch.
func LookupGrantsByInviteToken(ctx context.Context, orm *bun.DB, hash string) ([]*models.UserProject, error) {
	if orm == nil {
		return nil, errors.New("orm required")
	}
	if hash == "" {
		return nil, nil
	}
	var rows []*models.UserProject
	err := orm.NewSelect().
		Model(&rows).
		Where("invite_token = ?", hash).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ExpireInvitesByToken marks past-due invited rows expired.
func ExpireInvitesByToken(ctx context.Context, orm *bun.DB, hash, nowRFC3339 string) error {
	if orm == nil {
		return errors.New("orm required")
	}
	_, err := orm.NewUpdate().
		TableExpr(tableUserProjects).
		Set("invite_status = ?", models.InviteStatusExpired).
		Where("invite_token = ?", hash).
		Where("invite_status = ?", models.InviteStatusInvited).
		Where("invite_expires_at <> '' AND invite_expires_at < ?", nowRFC3339).
		Exec(ctx)
	return err
}

// AcceptGrantsByInviteToken sets matching invited (or already-accepted) rows to accepted.
func AcceptGrantsByInviteToken(ctx context.Context, orm *bun.DB, hash, acceptedAt string) error {
	if orm == nil {
		return errors.New("orm required")
	}
	_, err := orm.NewUpdate().
		TableExpr(tableUserProjects).
		Set("invite_status = ?", models.InviteStatusAccepted).
		Set("accepted_at = ?", acceptedAt).
		Where("invite_token = ?", hash).
		Where("invite_status IS NULL OR invite_status = '' OR invite_status = ? OR invite_status = ?",
			models.InviteStatusInvited, models.InviteStatusAccepted).
		Exec(ctx)
	return err
}

func (b *Base) GetUserProjectGrant(ctx context.Context, userID, projectID string) (*models.UserProject, error) {
	return GetUserProjectGrant(ctx, b.ORM, userID, projectID)
}

func (b *Base) LookupGrantsByInviteToken(ctx context.Context, hash string) ([]*models.UserProject, error) {
	return LookupGrantsByInviteToken(ctx, b.ORM, hash)
}

func (b *Base) ExpireInvitesByToken(ctx context.Context, hash, nowRFC3339 string) error {
	return ExpireInvitesByToken(ctx, b.ORM, hash, nowRFC3339)
}

func (b *Base) AcceptGrantsByInviteToken(ctx context.Context, hash, acceptedAt string) error {
	return AcceptGrantsByInviteToken(ctx, b.ORM, hash, acceptedAt)
}

// NowRFC3339 is UTC RFC3339 for invite expiry compares.
func NowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
