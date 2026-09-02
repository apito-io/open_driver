package sqlcommon

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testUserProjectsORM(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file:user-projects-grant?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	RegisterSystemSQLSchemaModels(db)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
CREATE TABLE system_users (
  id TEXT PRIMARY KEY,
  email TEXT,
  first_name TEXT,
  last_name TEXT,
  avatar TEXT
);
CREATE TABLE user_projects (
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL,
  role TEXT,
  permissions TEXT,
  invite_status TEXT,
  invited_at TEXT,
  accepted_at TEXT,
  invite_expires_at TEXT,
  invite_token TEXT,
  PRIMARY KEY (user_id, project_id)
);
`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO system_users (id, email, first_name, last_name) VALUES (?, ?, ?, ?)`,
		"u1", "a@example.test", "Ann", "One")
	require.NoError(t, err)
	return db
}

func TestUpsertListDeleteUserProjectGrant(t *testing.T) {
	db := testUserProjectsORM(t)
	ctx := context.Background()

	require.NoError(t, UpsertUserProjectGrant(ctx, db, &models.TeamMemberAddRequest{
		UserID:      "u1",
		ProjectID:   "p1",
		Role:        "editor",
		Permissions: []string{"contents", "settings"},
	}))

	members, err := ListProjectMembers(ctx, db, "p1")
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, "u1", members[0].ID)
	require.Equal(t, models.MembershipRoleTeam, members[0].ProjectAssignedRole)
	require.Equal(t, []string{"contents", "settings"}, members[0].ProjectAccessPermissions)

	require.NoError(t, UpsertUserProjectGrant(ctx, db, &models.TeamMemberAddRequest{
		UserID:      "u1",
		ProjectID:   "p1",
		Role:        "admin",
		Permissions: []string{"contents"},
	}))
	members, err = ListProjectMembers(ctx, db, "p1")
	require.NoError(t, err)
	require.Equal(t, models.MembershipRoleAdmin, members[0].ProjectAssignedRole)

	require.NoError(t, DeleteUserProjectGrant(ctx, db, "p1", "u1"))
	members, err = ListProjectMembers(ctx, db, "p1")
	require.NoError(t, err)
	require.Empty(t, members)
}

func TestDropObsoleteTeamOrgTables_Idempotent(t *testing.T) {
	db := testUserProjectsORM(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE teams (id TEXT PRIMARY KEY); CREATE TABLE user_teams (user_id TEXT, team_id TEXT);`)
	require.NoError(t, err)
	require.NoError(t, DropObsoleteTeamOrgTables(ctx, db))
	require.NoError(t, DropObsoleteTeamOrgTables(ctx, db))
	var n int
	err = db.NewSelect().ColumnExpr("count(*)").Table("user_projects").Scan(ctx, &n)
	require.NoError(t, err)
}

func TestInviteGrantLookupAccept(t *testing.T) {
	db := testUserProjectsORM(t)
	ctx := context.Background()
	raw, hash, err := models.NewInviteToken()
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	req := &models.TeamMemberAddRequest{
		UserID:      "u1",
		ProjectID:   "p1",
		Role:        "team",
		Permissions: []string{"contents"},
	}
	models.StampInviteOnRequest(req, nil, hash, time.Now().UTC(), 7*24*time.Hour)
	require.NoError(t, UpsertUserProjectGrant(ctx, db, req))

	found, err := LookupGrantsByInviteToken(ctx, db, hash)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, models.InviteStatusInvited, found[0].InviteStatus)

	members, err := ListProjectMembers(ctx, db, "p1")
	require.NoError(t, err)
	require.Equal(t, models.InviteStatusInvited, members[0].InviteStatus)

	require.NoError(t, AcceptGrantsByInviteToken(ctx, db, hash, time.Now().UTC().Format(time.RFC3339)))
	found, err = LookupGrantsByInviteToken(ctx, db, hash)
	require.NoError(t, err)
	require.Equal(t, models.InviteStatusAccepted, found[0].InviteStatus)
}
