package sqlcommon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

// UpsertUserProjectGrant writes one console membership row. Permissions are JSON.
func UpsertUserProjectGrant(ctx context.Context, orm *bun.DB, req *models.TeamMemberAddRequest) error {
	if orm == nil || req == nil {
		return fmt.Errorf("user project grant required")
	}
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.ProjectID) == "" {
		return fmt.Errorf("user_id and project_id are required")
	}
	role := models.NormalizeMembershipRole(req.Role, false)
	perms := models.MembershipPermissions(req.Permissions, false)
	raw, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	row := &models.UserProject{
		UserID:          req.UserID,
		ProjectID:       req.ProjectID,
		Role:            role,
		Permissions:     string(raw),
		InviteStatus:    req.InviteStatus,
		InvitedAt:       req.InvitedAt,
		AcceptedAt:      req.AcceptedAt,
		InviteExpiresAt: req.InviteExpiresAt,
		InviteToken:     req.InviteTokenHash,
	}
	q := orm.NewInsert().Model(row)
	stampInvite := strings.TrimSpace(req.InviteStatus) != ""
	switch orm.Dialect().Name() {
	case dialect.MySQL:
		if stampInvite {
			q = q.On("DUPLICATE KEY UPDATE role = VALUES(role), permissions = VALUES(permissions), invite_status = VALUES(invite_status), invited_at = VALUES(invited_at), accepted_at = VALUES(accepted_at), invite_expires_at = VALUES(invite_expires_at), invite_token = VALUES(invite_token)")
		} else {
			q = q.On("DUPLICATE KEY UPDATE role = VALUES(role), permissions = VALUES(permissions)")
		}
	default:
		if stampInvite {
			q = q.On("CONFLICT (user_id, project_id) DO UPDATE SET role = EXCLUDED.role, permissions = EXCLUDED.permissions, invite_status = EXCLUDED.invite_status, invited_at = EXCLUDED.invited_at, accepted_at = EXCLUDED.accepted_at, invite_expires_at = EXCLUDED.invite_expires_at, invite_token = EXCLUDED.invite_token")
		} else {
			q = q.On("CONFLICT (user_id, project_id) DO UPDATE SET role = EXCLUDED.role, permissions = EXCLUDED.permissions")
		}
	}
	_, err = q.Exec(ctx)
	return err
}

// DeleteUserProjectGrant removes one membership row.
func DeleteUserProjectGrant(ctx context.Context, orm *bun.DB, projectID, userID string) error {
	if orm == nil {
		return fmt.Errorf("orm required")
	}
	_, err := orm.NewDelete().
		Model((*models.UserProject)(nil)).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Exec(ctx)
	return err
}

// ListProjectMembers lists console operators on a project from user_projects.
func ListProjectMembers(ctx context.Context, orm *bun.DB, projectID string) ([]*models.SystemUser, error) {
	if orm == nil {
		return nil, fmt.Errorf("orm required")
	}
	type membership struct {
		UserID          string `bun:"user_id"`
		Role            string `bun:"role"`
		Permissions     string `bun:"permissions"`
		InviteStatus    string `bun:"invite_status"`
		InviteExpiresAt string `bun:"invite_expires_at"`
	}
	var rows []membership
	if err := orm.NewSelect().
		TableExpr("user_projects").
		Column("user_id", "role", "permissions", "invite_status", "invite_expires_at").
		Where("project_id = ?", projectID).
		Scan(ctx, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*models.SystemUser{}, nil
	}
	ids := make([]string, 0, len(rows))
	byID := make(map[string]membership, len(rows))
	for _, r := range rows {
		ids = append(ids, r.UserID)
		byID[r.UserID] = r
	}
	var users []*models.SystemUser
	if err := orm.NewSelect().
		Model(&users).
		Column("id", "email", "first_name", "last_name", "avatar").
		Where("id IN (?)", bun.In(ids)).
		Scan(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for _, u := range users {
		if u == nil {
			continue
		}
		m, ok := byID[u.ID]
		if !ok {
			continue
		}
		u.ProjectAssignedRole = m.Role
		u.InviteStatus = models.EffectiveInviteStatus(m.InviteStatus, m.InviteExpiresAt, now)
		u.InviteExpiresAt = m.InviteExpiresAt
		if strings.TrimSpace(m.Permissions) == "" {
			continue
		}
		var perms []string
		if json.Unmarshal([]byte(m.Permissions), &perms) == nil {
			u.ProjectAccessPermissions = perms
		}
	}
	return users, nil
}

// DropObsoleteTeamOrgTables drops the unused named Team/Organization graph.
// user_projects is kept. Safe on fresh installs (IF EXISTS).
func DropObsoleteTeamOrgTables(ctx context.Context, orm *bun.DB) error {
	if orm == nil {
		return fmt.Errorf("orm required")
	}
	tables := []string{
		"organization_teams",
		"organization_projects",
		"user_organizations",
		"user_teams",
		"team_projects",
		"project_teams",
		"team_metadata",
		"teams",
		"organizations",
	}
	for _, name := range tables {
		if _, err := orm.ExecContext(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
			return fmt.Errorf("drop %s: %w", name, err)
		}
	}
	return nil
}
