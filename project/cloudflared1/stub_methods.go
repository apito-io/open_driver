package cloudflared1

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/apito-io/open_driver/cfd1"
)

// stub methods — expanded incrementally; compile-time interface satisfaction via driver.go + this file.

func (d *Driver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteProject(ctx context.Context, projectID string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) TransferProject(ctx context.Context, userId, from, to string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	return false, cfd1.ErrNotImplemented
}
func (d *Driver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) RenameField(ctx context.Context, oldFieldName string, parentField string, param *models.CommonSystemParams) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) EnsureUsersTable(ctx context.Context) error                { return cfd1.ErrNotImplemented }
func (d *Driver) CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) ListProjectAuthUsersByOAuthSub(ctx context.Context, tenantID, provider, oauthSub string) ([]*models.ProjectAuthUser, error) {
	return nil, cfd1.ErrNotImplemented
}

func (d *Driver) SearchProjectAuthUsers(ctx context.Context, tenantID, q string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	return nil, 0, cfd1.ErrNotImplemented
}
func (d *Driver) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	return nil, cfd1.ErrNotImplemented
}
func (d *Driver) UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	return cfd1.ErrNotImplemented
}
func (d *Driver) DeleteProjectAuthUser(ctx context.Context, userID string) error { return cfd1.ErrNotImplemented }
