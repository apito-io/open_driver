//go:build !cloudflare

package project

import (
	"errors"
	"log"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/open_driver/project/bbolt"
	"github.com/apito-io/open_driver/project/mongo"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/telemetry"
)

// CreateOpenCoreProjectDriver opens the default open-core project driver without consulting DriverFactory.
func CreateOpenCoreProjectDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	log.Printf("Getting project driver: %s", engineConfig.Engine)

	var db interfaces.ProjectDBInterface
	var err error
	switch engineConfig.Engine {
	case _const.CoreDB, "coreDB":
		db, err = bbolt.GetBBoltDriver(conf, engineConfig)
	case _const.SQLiteDriver, _const.PostgreSQLDriver, _const.MySQLDriver, _const.MariaDBDriver:
		db, err = GetProjectSQLDriver(conf, engineConfig)
	case _const.MongoDBDriver:
		db, err = mongo.GetProjectMongoDriver(conf, engineConfig)
	default:
		return nil, errors.New("unsupported database driver passed")
	}
	if err != nil {
		return nil, err
	}
	db = telemetry.WrapProjectDBWithMetrics(conf, engineConfig.Engine, db)
	return db, nil
}
