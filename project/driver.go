package project

import (
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetProjectDriver(engineConfig *models.DriverCredentials, cfg *models.Config) (interfaces.ProjectDBInterface, error) {
	return GetProjectDriverWithConfig(cfg, engineConfig, nil)
}

func GetProjectDriverWithConfig(conf *models.Config, engineConfig *models.DriverCredentials, proExtras interface{}) (interfaces.ProjectDBInterface, error) {
	if conf != nil && conf.DriverFactory != nil {
		if pf, ok := conf.DriverFactory.(interfaces.ProjectDriverExtrasFactory); ok {
			if pf.SupportsEngine(engineConfig.Engine) {
				return pf.CreateProjectDriverWithExtras(conf, engineConfig, proExtras)
			}
		}
		if factory, ok := conf.DriverFactory.(interfaces.DatabaseDriverFactory); ok {
			if factory.SupportsEngine(engineConfig.Engine) {
				return factory.CreateProjectDriver(conf, engineConfig)
			}
		}
	}

	return CreateOpenCoreProjectDriver(conf, engineConfig)
}
