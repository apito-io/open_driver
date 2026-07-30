package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/uptrace/bun"
)

// GetProjectUsers retrieves metadata for multiple users in the project.
func (s *Driver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	results := make(map[string]*types.DefaultDocumentStructure)

	// In SQL, project users might be stored in a specific table
	var users []map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND id IN (?)"
	err := s.ORM.NewRaw(query, projectId, keys).Scan(ctx, &users)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		doc := &types.DefaultDocumentStructure{
			ID:   user["id"].(string),
			Type: "project_user",
			Data: user,
		}
		results[doc.ID] = doc
	}

	return results, nil
}

// GetProjectUser retrieves a user profile by phone, email, and project ID.
func (s *Driver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	var user map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND (phone = ? OR email = ?)"
	err := s.ORM.NewRaw(query, projectId, phone, email).Scan(ctx, &user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	doc := &types.DefaultDocumentStructure{
		ID:   user["id"].(string),
		Type: "project_user",
		Data: user,
	}

	return doc, nil
}

// GetLoggedInProjectUser retrieves the logged-in user profile for the project.
func (s *Driver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	var user map[string]interface{}
	query := "SELECT * FROM project_users WHERE project_id = ? AND user_id = ?"
	err := s.ORM.NewRaw(query, param.ProjectID, param.UserID).Scan(ctx, &user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	doc := &types.DefaultDocumentStructure{
		ID:   user["id"].(string),
		Type: "project_user",
		Data: user,
	}

	return doc, nil
}

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID.
func (s *Driver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	var revisions []*models.DocumentRevisionHistory

	query := `
		SELECT id, revision_at, status 
		FROM document_revisions 
		WHERE original_doc_id = ? OR id = ?
		ORDER BY revision_at DESC
	`

	var results []map[string]interface{}
	err := s.ORM.NewRaw(query, param.DocumentID, param.DocumentID).Scan(ctx, &results)
	if err != nil {
		return nil, err
	}

	for _, result := range results {
		revision := &models.DocumentRevisionHistory{
			ID:         result["id"].(string),
			RevisionAt: result["revision_at"].(string),
			Status:     result["status"].(string),
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

// AggregateDocOfProject aggregates numeric fields for ListCount.aggregate (SUM), matching
// Arango COLLECT AGGREGATE. Reuses list/count WHERE + relation EXISTS filters.
func (s *Driver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	if param == nil || param.Model == nil {
		return nil, errors.New("model is required for aggregate query")
	}
	if err := s.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}

	fieldNames := aggregateFieldNamesFromSelection(param)
	if len(fieldNames) == 0 {
		// No aggregate { … } selection — keep historical count-only shape.
		query, args, err := RootResolverQueryBuilder(s.Conf, param, true)
		if err != nil {
			return nil, err
		}
		var n int64
		if len(args) > 0 {
			if err := s.ORM.NewRaw(query, args...).Scan(ctx, &n); err != nil {
				return nil, err
			}
		} else if err := s.ORM.NewRaw(query).Scan(ctx, &n); err != nil {
			return nil, err
		}
		return map[string]interface{}{"count": n}, nil
	}

	selectExprs, err := AggregateSelectExpressions(param.Model, fieldNames)
	if err != nil {
		return nil, err
	}
	query, args, err := RootResolverAggregateQueryBuilder(s.Conf, param, selectExprs)
	if err != nil {
		return nil, err
	}

	// Scan one aggregate row into a map keyed by GraphQL field identifier.
	row := make(map[string]interface{}, len(fieldNames))
	var scanErr error
	if len(args) > 0 {
		scanErr = s.ORM.NewRaw(query, args...).Scan(ctx, &row)
	} else {
		scanErr = s.ORM.NewRaw(query).Scan(ctx, &row)
	}
	if scanErr != nil {
		// bun map scan may not bind aliased columns on all dialects — fall back to ordered NullFloat64.
		return s.scanAggregateRowOrdered(ctx, query, args, fieldNames)
	}
	if len(row) == 0 {
		return s.scanAggregateRowOrdered(ctx, query, args, fieldNames)
	}
	out := make(map[string]interface{}, len(fieldNames))
	for _, name := range fieldNames {
		if v, ok := row[name]; ok {
			out[name] = coerceAggregateNumber(v)
			continue
		}
		out[name] = float64(0)
	}
	return out, nil
}

func (s *Driver) scanAggregateRowOrdered(ctx context.Context, query string, args []interface{}, fieldNames []string) (map[string]interface{}, error) {
	dest := make([]interface{}, len(fieldNames))
	ptrs := make([]interface{}, len(fieldNames))
	for i := range fieldNames {
		var v sql.NullFloat64
		dest[i] = &v
		ptrs[i] = &v
	}
	var err error
	if len(args) > 0 {
		err = s.ORM.NewRaw(query, args...).Scan(ctx, ptrs...)
	} else {
		err = s.ORM.NewRaw(query).Scan(ctx, ptrs...)
	}
	if err != nil {
		return nil, fmt.Errorf("aggregate scan failed: %w", err)
	}
	out := make(map[string]interface{}, len(fieldNames))
	for i, name := range fieldNames {
		nf := dest[i].(*sql.NullFloat64)
		if nf != nil && nf.Valid {
			out[name] = nf.Float64
		} else {
			out[name] = float64(0)
		}
	}
	return out, nil
}

func coerceAggregateNumber(v interface{}) interface{} {
	switch n := v.(type) {
	case nil:
		return float64(0)
	case float64:
		return n
	case float32:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case int:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return float64(0)
		}
		return f
	case []byte:
		f, err := strconv.ParseFloat(string(n), 64)
		if err != nil {
			return float64(0)
		}
		return f
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return float64(0)
		}
		return f
	default:
		return v
	}
}

// AggregateDocOfProjectBytes aggregates the documents in the project and returns it as bytes.
func (s *Driver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	data, err := s.AggregateDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// NewInsertableRelations retrieves new insertable relations in the project.
func (s *Driver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	// Find existing relations
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(param.ForwardConnectionType.Model), utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	var existingIds []string
	query := fmt.Sprintf("SELECT %s_id FROM `%s` WHERE %s_id = ?",
		utility.PhysicalSQLTableName(param.BackwardConnectionType.Model),
		tableName,
		utility.PhysicalSQLTableName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &existingIds)
	if err != nil {
		return nil, err
	}

	// Filter out existing relations
	existingMap := make(map[string]bool)
	for _, id := range existingIds {
		existingMap[id] = true
	}

	var newRelations []string
	for _, actionID := range param.ActionIDs {
		if !existingMap[actionID] {
			newRelations = append(newRelations, actionID)
		}
	}

	return newRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation exists in the project.
func (s *Driver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(param.ForwardConnectionType.Model), utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s` WHERE %s_id = ?",
		tableName,
		utility.PhysicalSQLTableName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRelationIds retrieves the IDs of every document related to a document.
func (s *Driver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(param.ForwardConnectionType.Model), utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	var relationIds []string
	query := fmt.Sprintf("SELECT %s_id FROM `%s` WHERE %s_id = ?",
		utility.PhysicalSQLTableName(param.BackwardConnectionType.Model),
		tableName,
		utility.PhysicalSQLTableName(param.ForwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID).Scan(ctx, &relationIds)
	if err != nil {
		return nil, err
	}

	return relationIds, nil
}

// GetRelationDocument retrieves a relation document by ID.
func (s *Driver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(param.ForwardConnectionType.Model), utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	var result map[string]interface{}
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ? AND %s_id = ?",
		tableName,
		utility.PhysicalSQLTableName(param.ForwardConnectionType.Model),
		utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	err := s.ORM.NewRaw(query, param.ForwardConnectionID, param.ActionIDs[0]).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("relation not found")
		}
		return nil, err
	}

	// Convert to EdgeRelation structure
	relation := &models.EdgeRelation{
		Key:       result["id"].(string),
		From:      param.ForwardConnectionType.Model,
		To:        param.BackwardConnectionType.Model,
		FromID:    param.ForwardConnectionID,
		ToID:      param.ActionIDs[0],
		Relation:  param.ForwardConnectionType.Relation,
		CreatedAt: utility.GetCurrentTime(),
	}

	return relation, nil
}

// CreateRelation creates a relation in the project.
func (s *Driver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(relation.From), utility.PhysicalSQLTableName(relation.To))

	data := map[string]interface{}{
		"id": utility.NewID(),
		utility.PhysicalSQLTableName(relation.From) + "_id": relation.FromID,
		utility.PhysicalSQLTableName(relation.To) + "_id":   relation.ToID,
		"created_at": relation.CreatedAt,
	}

	_, err := s.ORM.NewInsert().Table(tableName).Model(&data).Exec(ctx)
	return err
}

// DeleteRelation deletes a relation in the project.
func (s *Driver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	tableName := fmt.Sprintf("%s_%s", utility.PhysicalSQLTableName(param.ForwardConnectionType.Model), utility.PhysicalSQLTableName(param.BackwardConnectionType.Model))

	_, err := s.ORM.NewDelete().Table(tableName).Where("id = ?", id).Exec(ctx)
	return err
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project.
func (s *Driver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Find all relation tables and delete relations involving this document
	// This is a simplified implementation - in practice you'd want to know all the relations for this model

	// Get all table names
	var tables []string
	err := s.ORM.NewRaw("SHOW TABLES").Scan(ctx, &tables)
	if err != nil {
		return err
	}

	// Look for relation tables (tables with two model names)
	for _, table := range tables {
		if strings.Contains(table, "_") && !strings.Contains(table, "meta") && !strings.Contains(table, "media") {
			// This looks like a relation table
			parts := strings.Split(table, "_")
			if len(parts) == 2 {
				// Try to delete relations where this document is referenced
				query1 := fmt.Sprintf("DELETE FROM `%s` WHERE %s_id = ?", table, parts[0])
				s.ORM.NewRaw(query1, param.DocumentID).Exec(ctx)

				query2 := fmt.Sprintf("DELETE FROM `%s` WHERE %s_id = ?", table, parts[1])
				s.ORM.NewRaw(query2, param.DocumentID).Exec(ctx)
			}
		}
	}

	return nil
}

// DeleteDocumentsFromProject deletes multiple documents from the project.
func (s *Driver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := utility.PhysicalSQLTableName(param.Model.Name)

	_, err := s.ORM.NewDelete().Table(tableName).Where("id IN (?)", param.DocumentIDs).Exec(ctx)
	if err != nil {
		return err
	}

	// Also delete from meta table
	_, err = s.ORM.NewDelete().Table("meta").Where("doc_id IN (?)", param.DocumentIDs).Exec(ctx)
	return err
}

// RenameModel renames a model table and related SQLite relation artifacts (pivots / FK columns).
// SQLite/libsql/Turso require ALTER TABLE … RENAME TO — MySQL's "RENAME TABLE" is a syntax error.
func (s *Driver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	oldTableName := utility.PhysicalSQLTableName(modelName)
	newTableName := utility.PhysicalSQLTableName(newName)
	if oldTableName == "" || newTableName == "" {
		return fmt.Errorf("RenameModel: invalid names %q → %q", modelName, newName)
	}
	if strings.EqualFold(oldTableName, newTableName) {
		return nil
	}

	exists, err := s.sqliteTableExists(ctx, oldTableName)
	if err != nil {
		return err
	}
	if exists {
		query := fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", oldTableName, newTableName)
		if _, err := s.ORM.NewRaw(query).Exec(ctx); err != nil {
			return fmt.Errorf("rename table %s → %s: %w", oldTableName, newTableName, err)
		}
	}

	if err := s.renameModelRelationArtifacts(ctx, project, modelName, newName); err != nil {
		return err
	}

	if project != nil && project.Schema != nil {
		for _, model := range project.Schema.Models {
			if model != nil && model.Name == modelName {
				model.Name = newName
				break
			}
		}
	}

	return nil
}

func (s *Driver) sqliteTableExists(ctx context.Context, name string) (bool, error) {
	var n int
	err := s.ORM.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(ctx, &n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Driver) sqliteColumnExists(ctx context.Context, table, column string) (bool, error) {
	var names []string
	// pragma_table_info is a table-valued function; selecting only `name` avoids
	// bun Scan mapping every PRAGMA table_info column onto a struct.
	err := s.ORM.NewRaw(`SELECT name FROM pragma_table_info(?)`, table).Scan(ctx, &names)
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if strings.EqualFold(n, column) {
			return true, nil
		}
	}
	return false, nil
}

// renameModelRelationArtifacts renames M:N pivot tables and has_one FK columns that
// embed the old physical model slug so connect/list keep working after rename.
func (s *Driver) renameModelRelationArtifacts(ctx context.Context, project *models.Project, oldName, newName string) error {
	if project == nil || project.Schema == nil {
		return nil
	}
	oldPhys := utility.PhysicalSQLTableName(oldName)
	newPhys := utility.PhysicalSQLTableName(newName)

	type pivotPair struct{ old, neu string }
	pivots := map[string]pivotPair{}
	type colRename struct{ table, oldCol, newCol string }
	var cols []colRename

	for _, model := range project.Schema.Models {
		if model == nil {
			continue
		}
		for _, conn := range model.Connections {
			if conn == nil || conn.Model == "" {
				continue
			}
			involvesOld := strings.EqualFold(model.Name, oldName) || strings.EqualFold(conn.Model, oldName)
			if !involvesOld {
				continue
			}

			oldPivot := relationPivotTableNameParts(model.Name, conn.Model, conn.KnownAs)
			newPivot := relationPivotTableNameParts(
				mapModelName(model.Name, oldName, newName),
				mapModelName(conn.Model, oldName, newName),
				conn.KnownAs,
			)
			if oldPivot != "" && newPivot != "" && oldPivot != newPivot {
				pivots[oldPivot] = pivotPair{old: oldPivot, neu: newPivot}
			}

			// has_one FK columns named {peer}_id / {peer}_as_{known}_id on this model table.
			if conn.Relation == "has_one" && strings.EqualFold(conn.Model, oldName) {
				oldCol := relationFKColumnNameParts(oldName, conn.KnownAs)
				newCol := relationFKColumnNameParts(newName, conn.KnownAs)
				if oldCol != "" && newCol != "" && oldCol != newCol {
					tbl := utility.PhysicalSQLTableName(mapModelName(model.Name, oldName, newName))
					cols = append(cols, colRename{table: tbl, oldCol: oldCol, newCol: newCol})
				}
			}
		}
	}

	for _, p := range pivots {
		ok, err := s.sqliteTableExists(ctx, p.old)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		destExists, err := s.sqliteTableExists(ctx, p.neu)
		if err != nil {
			return err
		}
		if destExists {
			continue
		}
		q := fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", p.old, p.neu)
		if _, err := s.ORM.NewRaw(q).Exec(ctx); err != nil {
			return fmt.Errorf("rename pivot %s → %s: %w", p.old, p.neu, err)
		}
		// Pivot column slugs follow physical model names.
		oldCol := oldPhys + "_id"
		newCol := newPhys + "_id"
		if oldCol != newCol {
			colOK, err := s.sqliteColumnExists(ctx, p.neu, oldCol)
			if err != nil {
				return err
			}
			if colOK {
				cq := fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", p.neu, oldCol, newCol)
				if _, err := s.ORM.NewRaw(cq).Exec(ctx); err != nil {
					return fmt.Errorf("rename pivot column %s.%s → %s: %w", p.neu, oldCol, newCol, err)
				}
			}
		}
	}

	for _, c := range cols {
		ok, err := s.sqliteTableExists(ctx, c.table)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		colOK, err := s.sqliteColumnExists(ctx, c.table, c.oldCol)
		if err != nil {
			return err
		}
		if !colOK {
			continue
		}
		destOK, err := s.sqliteColumnExists(ctx, c.table, c.newCol)
		if err != nil {
			return err
		}
		if destOK {
			continue
		}
		q := fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", c.table, c.oldCol, c.newCol)
		if _, err := s.ORM.NewRaw(q).Exec(ctx); err != nil {
			return fmt.Errorf("rename fk column %s.%s → %s: %w", c.table, c.oldCol, c.newCol, err)
		}
	}

	return nil
}

func mapModelName(name, oldName, newName string) string {
	if strings.EqualFold(name, oldName) {
		return newName
	}
	return name
}

// ConvertModel converts a model in the project.
func (s *Driver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// This could involve creating a new table with converted structure
	// For now, we'll create a backup table
	oldTableName := utility.PhysicalSQLTableName(modelName)
	newTableName := fmt.Sprintf("converted_%s", oldTableName)

	query := fmt.Sprintf("CREATE TABLE `%s` AS SELECT * FROM `%s`", newTableName, oldTableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// RenameField renames a field in a model along with its data key.
func (s *Driver) RenameField(ctx context.Context, oldFieldName string, parentField string, param *models.CommonSystemParams) error {

	// since for repeated groups and for objects we dont need to rename the field, we only need to rename the data key
	if parentField != "" {
		return nil
	}

	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	newFieldName := param.FieldInfo.Identifier

	var query string
	switch s.DriverCredential.Engine {
	case _const.MySQLDriver, _const.MariaDBDriver:
		// MySQL/MariaDB require full CHANGE COLUMN syntax with type.
		query = fmt.Sprintf("ALTER TABLE `%s` CHANGE COLUMN `%s` `%s` %s", tableName, oldFieldName, newFieldName, "TEXT")
	case _const.PostgreSQLDriver:
		query = fmt.Sprintf(
			"ALTER TABLE %s RENAME COLUMN %s TO %s",
			QuotePGIdent(tableName),
			QuotePGIdent(oldFieldName),
			QuotePGIdent(newFieldName),
		)
	default:
		// PostgreSQL/SQLite/libsql/Turso support RENAME COLUMN.
		query = fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tableName, oldFieldName, newFieldName)
	}
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// DropModel drops a model from the project.
func (s *Driver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	tableName := utility.PhysicalSQLTableName(modelName)

	query := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return err
	}

	// Remove from project schema
	var updatedModels []*models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name != modelName {
			updatedModels = append(updatedModels, model)
		}
	}
	project.Schema.Models = updatedModels

	return nil
}

// buildCreateIndexSQL returns engine-specific CREATE INDEX IF NOT EXISTS DDL (no execution).
func (s *Driver) buildCreateIndexSQL(param *models.CommonSystemParams, fieldName string, parent_field string) (string, error) {
	_ = parent_field
	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	indexName := fmt.Sprintf("idx_%s_%s", tableName, fieldName)
	if s.DriverCredential == nil {
		return "", errors.New("CreateIndex: nil driver credentials")
	}
	switch s.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
			QuotePGIdent(indexName), QuotePGIdent(tableName), QuotePGIdent(fieldName)), nil
	case _const.MySQLDriver, _const.MariaDBDriver:
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS `%s` ON `%s` (`%s`)",
			strings.ReplaceAll(indexName, "`", "``"),
			strings.ReplaceAll(tableName, "`", "``"),
			strings.ReplaceAll(fieldName, "`", "``")), nil
	default:
		return fmt.Sprintf("CREATE INDEX IF NOT EXISTS `%s` ON `%s` (`%s`)", indexName, tableName, fieldName), nil
	}
}

// execCreateIndexDDL runs CREATE INDEX on a bun.DB or bun.Tx (no ANALYZE).
func (s *Driver) execCreateIndexDDL(ctx context.Context, db bun.IDB, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	q, err := s.buildCreateIndexSQL(param, fieldName, parent_field)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(q).Exec(ctx)
	return err
}

// CreateIndex creates an index for a model in the project.
func (s *Driver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	if err := s.execCreateIndexDDL(ctx, s.ORM, param, fieldName, parent_field); err != nil {
		return err
	}
	RunAnalyzeAfterIndexDDL(ctx, s)
	return nil
}

// DropIndex drops an index from a model in the project.
func (s *Driver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	tableName := utility.PhysicalSQLTableName(param.Model.Name)

	query := fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	return err
}

// AddTeamMetaInfo adds metadata information for a team in the project.
func (s *Driver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	// Get additional team metadata for each user
	var userIds []string
	for _, user := range docs {
		userIds = append(userIds, user.ID)
	}

	if len(userIds) == 0 {
		return docs, nil
	}

	// Query team metadata
	var teamData []map[string]interface{}
	query := "SELECT * FROM team_members WHERE user_id IN (?)"
	err := s.ORM.NewRaw(query, userIds).Scan(ctx, &teamData)
	if err != nil {
		return docs, nil // Return original docs if metadata query fails
	}

	// Map team data to users
	teamMap := make(map[string]map[string]interface{})
	for _, data := range teamData {
		if userID, ok := data["user_id"].(string); ok {
			teamMap[userID] = data
		}
	}

	// Enhance user data with team metadata
	for _, user := range docs {
		if teamInfo, exists := teamMap[user.ID]; exists {
			// Add team-specific metadata to user
			if role, ok := teamInfo["team_role"].(string); ok {
				user.ProjectAssignedRole = role
			}
		}
	}

	return docs, nil
}

// DeleteMediaFile deletes a media file from the project files table.
func (s *Driver) DeleteMediaFile(ctx context.Context, param models.CommonSystemParams) error {
	if strings.TrimSpace(param.DocumentID) == "" {
		return nil
	}
	return s.DeleteProjectFiles(ctx, []string{param.DocumentID})
}

// DuplicateModel duplicates a model in the project.
func (s *Driver) DuplicateModel(ctx context.Context, project *models.Project, modelName, newName string) (*models.ProjectSchema, error) {
	// Find the original model
	var originalModel *models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name == modelName {
			originalModel = model
			break
		}
	}

	if originalModel == nil {
		return nil, errors.New("model not found")
	}

	// Create a copy of the model
	newModel := &models.ModelType{
		Name:   newName,
		Fields: make([]*models.FieldInfo, len(originalModel.Fields)),
	}

	// Deep copy fields
	for i, field := range originalModel.Fields {
		newModel.Fields[i] = &models.FieldInfo{
			Identifier:  field.Identifier,
			Description: field.Description,
			InputType:   field.InputType,
			FieldType:   field.FieldType,
			Validation:  field.Validation,
			Serial:      field.Serial,
			Label:       field.Label,
		}
	}

	// Add to project schema
	project.Schema.Models = append(project.Schema.Models, newModel)

	// Create the database table
	oldTableName := utility.PhysicalSQLTableName(modelName)
	newTableName := utility.PhysicalSQLTableName(newName)

	query := fmt.Sprintf("CREATE TABLE `%s` LIKE `%s`", newTableName, oldTableName)
	_, err := s.ORM.NewRaw(query).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return project.Schema, nil
}
