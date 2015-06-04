package main

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/cli/cf/api/applications"
	"github.com/cloudfoundry/cli/cf/api/authentication"
	"github.com/cloudfoundry/cli/cf/configuration/core_config"
	"github.com/cloudfoundry/cli/cf/configuration/config_helpers"
	"github.com/cloudfoundry/cli/cf/i18n"
	"github.com/cloudfoundry/cli/cf/i18n/detection"
	"github.com/cloudfoundry/cli/cf/models"
	"github.com/cloudfoundry/cli/cf/net"
	"github.com/cloudfoundry/cli/cf/trace"
)

type CloudController struct {
	config core_config.Repository
	gateway net.Gateway
	appRepo applications.CloudControllerApplicationRepository
}

func NewCloudController() (cc CloudController) {

	errorHandler := func(err error) {
		if err != nil {
			fmt.Sprintf("Config error: %s", err)
		}
	}
	cc.config = core_config.NewRepositoryFromFilepath(config_helpers.DefaultFilePath(), errorHandler)
	cc.gateway = net.NewCloudControllerGateway(cc.config, time.Now, nil)
	cc.gateway.SetTokenRefresher(authentication.NewUAAAuthenticationRepository(cc.gateway, cc.config))
	cc.appRepo = applications.NewCloudControllerApplicationRepository(cc.config, cc.gateway)

	// I18N usage in the library will cause the app to crash unless this is initialized
	i18n.T = i18n.Init(cc.config, &detection.JibberJabberDetector{})
	trace.Logger = trace.NewLogger("true")

	return
}

func (cc *CloudController) GetApplication(appName string) (app models.Application, err error) {
	app, err = cc.appRepo.Read(appName)
	return
}

func (cc *CloudController) UpdateApplication(appGuid string, params models.AppParams) (app models.Application, err error) {
	return cc.appRepo.Update(appGuid, params)
}