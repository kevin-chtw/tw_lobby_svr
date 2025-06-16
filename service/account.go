package service

import (
	"context"

	"github.com/kevin-chtw/tw_proto/proto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
)

type AccountSvc struct {
	component.Base
	app pitaya.Pitaya
}

func NewAccountSvc(app pitaya.Pitaya) *AccountSvc {
	return &AccountSvc{
		app: app,
	}
}

func (s *AccountSvc) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.CommonResponse, error) {
	logrus.Debugf("Received register request: %v", req)
	logrus.Debugf("Registering user: %s", req.Account)

	rsp := &proto.CommonResponse{
		Err: proto.ErrCode_OK,
	}

	return rsp, nil
}
