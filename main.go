package main

import (
	"strings"

	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_island_svr/service"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"github.com/topfreegames/pitaya/v3/pkg/serialize"
)

var app pitaya.Pitaya

func main() {
	// queue.SendCommonMsgQueue("Dau", []string{"test 10000"})
	// queue.SendCommonMsgQueue("充值", []string{"充值总额10000,充值次数5000"})
	serverType := "island"
	pitaya.SetLogger(utils.Logger(logrus.DebugLevel))

	config := config.NewDefaultPitayaConfig()
	config.SerializerType = uint16(serialize.PROTOBUF)
	config.Handler.Messages.Compression = false
	builder := pitaya.NewBuilder(false, serverType, pitaya.Cluster, map[string]string{}, *config)
	app = builder.Build()

	defer app.Shutdown()
	initServices()
	logger.Log.Infof("Pitaya server of type %s started", serverType)
	app.Start()
}

func initServices() {
	player := service.NewPlayer(app)
	app.Register(player, component.WithName("player"), component.WithNameFunc(strings.ToLower))

	remote := service.NewRemote(app)
	app.RegisterRemote(remote, component.WithName("remote"), component.WithNameFunc(strings.ToLower))
}
