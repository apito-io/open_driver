package cloudflared1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
)

const filesTableDDL = `
CREATE TABLE IF NOT EXISTS files(
	id TEXT NOT NULL PRIMARY KEY,
	project_id TEXT,
	file_type TEXT NOT NULL,
	file_name TEXT NOT NULL,
	file_extension TEXT,
	content_type TEXT,
	size INTEGER NOT NULL,
	storage_key TEXT NOT NULL,
	url TEXT,
	created_by TEXT,
	created_at TEXT,
	updated_at TEXT
);`

func (d *Driver) EnsureFilesTable(ctx context.Context) error {
	_, err := d.DB.Exec(ctx, filesTableDDL)
	return err
}

func (d *Driver) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	if file == nil {
		return nil, fmt.Errorf("file is required")
	}
	if err := d.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(file.CreatedAt) == "" {
		file.CreatedAt = now
	}
	if strings.TrimSpace(file.UpdatedAt) == "" {
		file.UpdatedAt = now
	}
	_, err := d.DB.Exec(ctx, `INSERT INTO files (
		id, project_id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_by, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.ID, file.ProjectID, file.FileType, file.FileName, file.FileExtension, file.ContentType, file.Size,
		file.StorageKey, file.URL, file.CreatedBy, file.CreatedAt, file.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (d *Driver) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return nil, fmt.Errorf("file id is required")
	}
	row, err := d.DB.QueryRow(ctx, `SELECT id, project_id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_by, created_at, updated_at FROM files WHERE id = ?`, fileID)
	if err != nil || row == nil {
		return nil, err
	}
	return rowToProjectFile(row), nil
}

func (d *Driver) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	if err := d.EnsureFilesTable(ctx); err != nil {
		return nil, err
	}
	fileType, limit, offset := models.SystemFileListParams(param)
	query := `SELECT id, project_id, file_type, file_name, file_extension, content_type, size, storage_key, url, created_by, created_at, updated_at FROM files`
	args := make([]any, 0, 3)
	if fileType != "" {
		query += ` WHERE file_type = ?`
		args = append(args, fileType)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := d.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	countQuery := `SELECT COUNT(*) AS n FROM files`
	countArgs := []any{}
	if fileType != "" {
		countQuery += ` WHERE file_type = ?`
		countArgs = append(countArgs, fileType)
	}
	countRow, err := d.DB.QueryRow(ctx, countQuery, countArgs...)
	if err != nil {
		return nil, err
	}
	total := int64(0)
	if countRow != nil {
		switch v := countRow["n"].(type) {
		case int64:
			total = v
		case float64:
			total = int64(v)
		case string:
			fmt.Sscanf(v, "%d", &total)
		}
	}
	results := make([]*models.ProjectFile, 0, len(rows))
	for _, row := range rows {
		results = append(results, rowToProjectFile(row))
	}
	return &models.SearchResponse[models.ProjectFile]{Results: results, Total: total}, nil
}

func (d *Driver) DeleteProjectFiles(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = strings.TrimSpace(id)
	}
	query := fmt.Sprintf(`DELETE FROM files WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := d.DB.Exec(ctx, query, args...)
	return err
}

func (d *Driver) SumProjectFilesSize(ctx context.Context) (int64, error) {
	if err := d.EnsureFilesTable(ctx); err != nil {
		return 0, err
	}
	row, err := d.DB.QueryRow(ctx, `SELECT COALESCE(SUM(size), 0) AS total FROM files`)
	if err != nil || row == nil {
		return 0, err
	}
	switch v := row["total"].(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case string:
		var total int64
		fmt.Sscanf(v, "%d", &total)
		return total, nil
	default:
		return 0, nil
	}
}

func rowToProjectFile(row map[string]any) *models.ProjectFile {
	if row == nil {
		return nil
	}
	file := &models.ProjectFile{
		ID:            asString(row["id"]),
		ProjectID:     asString(row["project_id"]),
		FileType:      asString(row["file_type"]),
		FileName:      asString(row["file_name"]),
		FileExtension: asString(row["file_extension"]),
		ContentType:   asString(row["content_type"]),
		StorageKey:    asString(row["storage_key"]),
		URL:           asString(row["url"]),
		CreatedBy:     asString(row["created_by"]),
		CreatedAt:     asString(row["created_at"]),
		UpdatedAt:     asString(row["updated_at"]),
	}
	switch v := row["size"].(type) {
	case int64:
		file.Size = v
	case float64:
		file.Size = int64(v)
	case string:
		fmt.Sscanf(v, "%d", &file.Size)
	}
	return file
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}
