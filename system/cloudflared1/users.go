package cloudflared1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"gitlab.com/apito.io/open_driver/system/bootstrapmeta"
)

func rowToSystemUser(row map[string]any) (*models.SystemUser, error) {
	if row == nil {
		return nil, sql.ErrNoRows
	}
	u := &models.SystemUser{
		ID:     asString(row["id"]),
		Email:  asString(row["email"]),
		Secret: asString(row["password"]),
	}
	if raw := asString(row["data"]); raw != "" && raw != "{}" {
		var overlay models.SystemUser
		if err := json.Unmarshal([]byte(raw), &overlay); err != nil {
			return nil, err
		}
		if overlay.Username != "" {
			u.Username = overlay.Username
		}
		if overlay.FirstName != "" {
			u.FirstName = overlay.FirstName
		}
		if overlay.LastName != "" {
			u.LastName = overlay.LastName
		}
		u.IsAdmin = overlay.IsAdmin
		if len(overlay.AdministrativePermissions) > 0 {
			u.AdministrativePermissions = overlay.AdministrativePermissions
		}
		if overlay.DefaultTeamID != "" {
			u.DefaultTeamID = overlay.DefaultTeamID
		}
		if overlay.DefaultOrganizationID != "" {
			u.DefaultOrganizationID = overlay.DefaultOrganizationID
		}
		if overlay.CreatedAt != "" {
			u.CreatedAt = overlay.CreatedAt
		}
		if overlay.UpdatedAt != "" {
			u.UpdatedAt = overlay.UpdatedAt
		}
		u.RefreshToken = overlay.RefreshToken
		u.AccessToken = overlay.AccessToken
		u.LastLoggedIn = overlay.LastLoggedIn
	}
	if u.Secret == "" {
		u.Secret = overlaySecret(row)
	}
	return u, nil
}

func overlaySecret(row map[string]any) string { return asString(row["password"]) }

func systemUserDataJSON(u *models.SystemUser) (string, error) {
	if u == nil {
		return "{}", nil
	}
	cp := *u
	cp.Secret = ""
	payload, err := json.Marshal(cp)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (d *Driver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	row, err := d.DB.QueryRow(ctx, `SELECT id, email, password, data FROM system_users WHERE email = ?`, strings.TrimSpace(email))
	if err != nil {
		return nil, err
	}
	return rowToSystemUser(row)
}

func (d *Driver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user == nil {
		return nil, cfd1Err("create system user: nil")
	}
	if user.ID == "" {
		user.ID = utility.NewID()
	}
	now := utility.GetCurrentTime()
	if user.CreatedAt == "" {
		user.CreatedAt = now
	}
	if user.UpdatedAt == "" {
		user.UpdatedAt = now
	}
	data, err := systemUserDataJSON(user)
	if err != nil {
		return nil, err
	}
	_, err = d.DB.Exec(ctx, `INSERT INTO system_users (id, email, password, data) VALUES (?, ?, ?, ?)`,
		user.ID, user.Email, user.Secret, data)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (d *Driver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return cfd1Err("update system user: id required")
	}
	user.UpdatedAt = utility.GetCurrentTime()
	data, err := systemUserDataJSON(user)
	if err != nil {
		return err
	}
	_, err = d.DB.Exec(ctx, `UPDATE system_users SET email = ?, password = ?, data = ? WHERE id = ?`,
		user.Email, user.Secret, data, user.ID)
	return err
}

func (d *Driver) SearchSystemUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	if param == nil || (!param.AllowUnscopedSystemUserList && !param.HasSystemUserSearchFilter()) {
		return &models.SearchResponse[models.SystemUser]{Results: []*models.SystemUser{}}, nil
	}
	email := extractEmailEq(param)
	var rows []map[string]any
	var err error
	if email != "" {
		rows, err = d.DB.Query(ctx, `SELECT id, email, password, data FROM system_users WHERE email = ? LIMIT 100`, email)
	} else {
		rows, err = d.DB.Query(ctx, `SELECT id, email, password, data FROM system_users LIMIT 100`)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*models.SystemUser, 0, len(rows))
	for _, row := range rows {
		u, uerr := rowToSystemUser(row)
		if uerr != nil {
			return nil, uerr
		}
		out = append(out, u)
	}
	return &models.SearchResponse[models.SystemUser]{Results: out}, nil
}

func extractEmailEq(param *models.CommonSystemParams) string {
	if param == nil || param.ResolveParams == nil {
		return ""
	}
	where, ok := param.ResolveParams.Args["where"].(map[string]interface{})
	if !ok {
		return ""
	}
	emailField, ok := where["email"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, ok := emailField["eq"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func (d *Driver) ensureBootstrapAdmin(ctx context.Context) error {
	u, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: check admin user: %w", err)
	}
	if u != nil {
		return nil
	}
	hash, err := bootstrapmeta.HashDefaultAdminPassword()
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}
	id := utility.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	user := &models.SystemUser{
		ID:                        id,
		Email:                     bootstrapmeta.AdminEmail,
		Username:                  utility.NewID(),
		FirstName:                 bootstrapmeta.AdminFirstName,
		LastName:                  bootstrapmeta.AdminLastName,
		Secret:                    string(hash),
		IsAdmin:                   true,
		IsSuperAdmin:              true,
		IsActive:                  true,
		AdministrativePermissions: []string{"all"},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if _, err := d.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}
	bootstrapmeta.LogDefaultAdminCredentials(bootstrapmeta.AdminEmail, bootstrapmeta.DefaultAdminPass)
	return nil
}
