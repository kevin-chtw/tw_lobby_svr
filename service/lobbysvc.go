package service

import (
	"context"
	"errors"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	e "github.com/topfreegames/pitaya/v3/pkg/errors"
)

type LobbySvc struct {
	component.Base
	app pitaya.Pitaya
}

func NewLobbySvc(app pitaya.Pitaya) *LobbySvc {
	return &LobbySvc{
		app: app,
	}
}

func (l *LobbySvc) PlayerMsg(ctx context.Context, req *cproto.LobbyReq) (*cproto.CommonResponse, error) {
	logrus.Debugf("PlayerMsg: %v", req)

	if req.LoginReq != nil {
		return nil, l.handleLogin(ctx, req.LoginReq)
	}

	if req.RegisterReq != nil {
		return nil, l.handleRegister(ctx, req.RegisterReq)
	}

	return &cproto.CommonResponse{
		Err: 0,
	}, nil
}

func (l *LobbySvc) handleLogin(ctx context.Context, req *cproto.LoginReq) error {
	s := l.app.GetSessionFromCtx(ctx)

	// 验证账号密码
	valid, userID, err := l.verifyAccount(ctx, req.Account, req.Password)
	if err != nil {
		logrus.Errorf("account verification failed: %v", err)
		return pitaya.Error(err, e.ErrInternalCode)
	}

	if !valid {
		return pitaya.Error(errors.New("invalid account or password"), e.ErrNotFoundCode, map[string]string{
			"account": req.Account,
		})
	}

	// 绑定用户会话
	if err := s.Bind(ctx, userID); err != nil {
		logrus.Errorf("failed to bind session: %v", err)
		return pitaya.Error(err, e.ErrInternalCode)
	}

	// 返回登录成功响应
	return s.Push("lobbymsg", &cproto.LobbyAck{
		LoginAck: &cproto.LoginAck{
			Serverid: l.app.GetServerID(),
			Userid:   userID,
		},
	})
}

func (l *LobbySvc) verifyAccount(ctx context.Context, account, password string) (bool, string, error) {
	// 调用数据库服务验证账号
	rsp := &cproto.LobbyAck{}
	err := l.app.RPC(ctx, "db.account.verify", rsp, &cproto.LobbyReq{
		LoginReq: &cproto.LoginReq{
			Account:  account,
			Password: password,
		},
	})

	if err != nil {
		return false, "", err
	}

	return true, rsp.LoginAck.Userid, nil
}

func (l *LobbySvc) createAccount(ctx context.Context, account, password string) (string, error) {
	// 调用数据库服务创建账号
	rsp := &cproto.LobbyAck{}
	err := l.app.RPC(ctx, "db.account.create", rsp, &cproto.LobbyReq{
		RegisterReq: &cproto.RegisterReq{
			Account:  account,
			Password: password,
		},
	})

	if rsp.RegisterAck == nil || err != nil {
		return "", err
	}

	return rsp.RegisterAck.Userid, nil
}

func (l *LobbySvc) handleRegister(ctx context.Context, req *cproto.RegisterReq) error {
	s := l.app.GetSessionFromCtx(ctx)

	// 创建新账号
	userID, err := l.createAccount(ctx, req.Account, req.Password)
	if err != nil {
		logrus.Errorf("account creation failed: %v", err)
		return pitaya.Error(err, e.ErrInternalCode)
	}

	// 绑定用户会话
	if err := s.Bind(ctx, userID); err != nil {
		logrus.Errorf("failed to bind session: %v", err)
		return pitaya.Error(err, e.ErrInternalCode)
	}

	// 返回注册成功响应
	return s.Push("lobbymsg", &cproto.LobbyAck{
		RegisterAck: &cproto.RegisterAck{
			Serverid: l.app.GetServerID(),
			Userid:   userID,
		},
	})
}
