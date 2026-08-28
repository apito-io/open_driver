package sqlcommon

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// SearchSystemUsers lists console operators. Unscoped dumps require
// AllowUnscopedSystemUserList (searchPlatformAccounts). Team invite search must
// supply an email/name filter or it returns empty.
func SearchSystemUsers(ctx context.Context, db *bun.DB, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	if db == nil {
		return &models.SearchResponse[models.SystemUser]{}, nil
	}
	allowUnscoped := param != nil && param.AllowUnscopedSystemUserList
	hasFilter := param != nil && param.HasSystemUserSearchFilter()
	if !allowUnscoped && !hasFilter {
		return &models.SearchResponse[models.SystemUser]{Results: []*models.SystemUser{}}, nil
	}

	var users []*models.SystemUser
	query := db.NewSelect().Model(&users)

	if email := ""; param != nil {
		if v := strings.TrimSpace(param.Email); v != "" {
			email = v
		} else {
			email = param.SystemUserWhereEq("email")
		}
		if email != "" {
			query = query.Where("LOWER(email) = LOWER(?)", email)
		} else if contains := param.SystemUserWhereContains("email"); contains != "" {
			query = query.Where("email LIKE ?", "%"+contains+"%")
		}
		if eq := param.SystemUserWhereEq("first_name"); eq != "" {
			query = query.Where("LOWER(first_name) = LOWER(?)", eq)
		} else if contains := param.SystemUserWhereContains("first_name"); contains != "" {
			query = query.Where("first_name LIKE ?", "%"+contains+"%")
		}
		if eq := param.SystemUserWhereEq("last_name"); eq != "" {
			query = query.Where("LOWER(last_name) = LOWER(?)", eq)
		} else if contains := param.SystemUserWhereContains("last_name"); contains != "" {
			query = query.Where("last_name LIKE ?", "%"+contains+"%")
		}
		if eq := param.SystemUserWhereEq("username"); eq != "" {
			query = query.Where("LOWER(username) = LOWER(?)", eq)
		} else if contains := param.SystemUserWhereContains("username"); contains != "" {
			query = query.Where("username LIKE ?", "%"+contains+"%")
		}
		if q := param.SystemUserSearchQ(); q != "" {
			like := "%" + q + "%"
			query = query.Where("(email LIKE ? OR first_name LIKE ? OR last_name LIKE ?)", like, like, like)
		}
	}

	limit, page := 100, 1
	if param != nil {
		limit, page = param.SystemUserSearchLimitPage(100, 200)
	}
	query = query.Limit(limit).Offset((page - 1) * limit)

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return &models.SearchResponse[models.SystemUser]{Results: users}, nil
}

// CountUserProjects returns project membership counts for the given user IDs.
func CountUserProjects(ctx context.Context, db *bun.DB, userIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(userIDs))
	if db == nil || len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		UserID string `bun:"user_id"`
		N      int    `bun:"n"`
	}
	err := db.NewSelect().
		Model((*models.UserProject)(nil)).
		Column("user_id").
		ColumnExpr("count(*) AS n").
		Where("user_id IN (?)", bun.In(userIDs)).
		Group("user_id").
		Scan(ctx, &rows)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		out[r.UserID] = r.N
	}
	return out, nil
}
