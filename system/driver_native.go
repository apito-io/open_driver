//go:build !cloudflare

package system

import (
	"log"

	_const "github.com/apito-io/engine/const"
	"gitlab.com/apito.io/open_driver/system/bbolt"
	mongodb "gitlab.com/apito.io/open_driver/system/mongodb"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateOpenCoreSystemDriver opens the default open-core system driver without consulting DriverFactory.
func CreateOpenCoreSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {
	log.Printf("Getting system driver: %s", engineConfig.Engine)

	var db interfaces.ApitoSystemDB
	var err error
	switch engineConfig.Engine {
	case _const.CoreDB, "coreDB":
		db, err = bbolt.GetSystemBBoltDriver(conf, engineConfig, nil)
	case _const.MongoDBDriver:
		db, err = mongodb.GetSystemMongoDriver(engineConfig, conf)
	case _const.SQLiteDriver, _const.PostgreSQLDriver, _const.MySQLDriver, _const.MariaDBDriver:
		db, err = GetSQLSystemDriver(conf, engineConfig)
	default:
		db, err = bbolt.GetSystemBBoltDriver(conf, engineConfig, nil)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}
