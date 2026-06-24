package system

import (
	"errors"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {
	if engineConfig == nil {
		return nil, errors.New("please select a system database engine")
	}

	if conf != nil && conf.DriverFactory != nil {
		if factory, ok := conf.DriverFactory.(interfaces.DatabaseDriverFactory); ok {
			if factory.SupportsEngine(engineConfig.Engine) {
				return factory.CreateSystemDriver(conf, engineConfig)
			}
		}
	}

	return CreateOpenCoreSystemDriver(conf, engineConfig)
}
