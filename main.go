package main

import (
	"strings"

	"github.com/kevin-chtw/tw_lobby_svr/service"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
	"github.com/topfreegames/pitaya/v3/pkg/session"
)

var app pitaya.Pitaya

func main() {
	serverType := "lobby"

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetReportCaller(true)

	builder := pitaya.NewBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config.NewDefaultPitayaConfig())
	app = builder.Build()

	defer app.Shutdown()

	logrus.Infof("Pitaya server of type %s started", serverType)
	initServices(builder.SessionPool)
	app.Start()
}

func initServices(sessionPool session.SessionPool) {
	playerSvc := service.NewPlayerSvc(app, sessionPool)
	app.Register(playerSvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
	app.RegisterRemote(playerSvc, component.WithName("player"), component.WithNameFunc(strings.ToLower))
}
