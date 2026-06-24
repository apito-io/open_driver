package factory

import (
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/project"
	"gitlab.com/apito.io/open_driver/system"
)

// DefaultFactory implements open-core database drivers (BBolt, SQL, Mongo).
type DefaultFactory struct{}

func NewDefaultFactory() interfaces.DatabaseDriverFactory {
	return &DefaultFactory{}
}

func (f *DefaultFactory) SupportsEngine(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case strings.ToLower(_const.CoreDB), "coredb":
		return true
	case strings.ToLower(_const.SQLiteDriver), strings.ToLower(_const.PostgreSQLDriver),
		strings.ToLower(_const.MySQLDriver), strings.ToLower(_const.MariaDBDriver):
		return true
	case strings.ToLower(_const.MongoDBDriver):
		return true
	default:
		return false
	}
}

func (f *DefaultFactory) CreateSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {
	return system.CreateOpenCoreSystemDriver(conf, engineConfig)
}

func (f *DefaultFactory) CreateProjectDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	return project.CreateOpenCoreProjectDriver(conf, engineConfig)
}
