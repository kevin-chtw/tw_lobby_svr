package service

import (
	"context"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	e "github.com/topfreegames/pitaya/v3/pkg/errors"
	"github.com/topfreegames/pitaya/v3/pkg/session"
)

type PlayerSvc struct {
	component.Base
	app         pitaya.Pitaya
	sessionPool session.SessionPool
}

func NewPlayerSvc(app pitaya.Pitaya, sessionPool session.SessionPool) *PlayerSvc {
	return &PlayerSvc{
		app:         app,
		sessionPool: sessionPool,
	}
}

func (l *PlayerSvc) Message(ctx context.Context, req *cproto.LobbyReq) (*cproto.CommonResponse, error) {
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

func (l *PlayerSvc) handleLogin(ctx context.Context, req *cproto.LoginReq) error {
	s := l.app.GetSessionFromCtx(ctx)

	rsp := &sproto.GetPlayerAck{}
	if err := l.app.RPC(ctx, "db.player.get", rsp, &sproto.GetPlayerReq{
		Account:  req.Account,
		Password: req.Password,
	}); err != nil {
		return err
	}

	if old := l.sessionPool.GetSessionByUID(rsp.Userid); old != nil {
		// 踢掉旧玩家
		old.Kick(context.Background())
	}

	// 绑定用户会话
	if err := s.Bind(ctx, rsp.Userid); err != nil {
		logrus.Errorf("failed to bind session: %v,uid:%v", err, rsp.Userid)
		return pitaya.Error(err, e.ErrInternalCode)
	}

	// 返回登录成功响应
	return s.Push("lobbymsg", &cproto.LobbyAck{
		LoginAck: &cproto.LoginAck{
			Serverid: l.app.GetServerID(),
			Userid:   rsp.Userid,
		},
	})
}

func (l *PlayerSvc) createAccount(ctx context.Context, account, password string) (string, error) {
	// 调用数据库服务创建账号
	rsp := &cproto.LobbyAck{}
	err := l.app.RPC(ctx, "db.player.create", rsp, &cproto.LobbyReq{
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

func (l *PlayerSvc) handleRegister(ctx context.Context, req *cproto.RegisterReq) error {
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

// PlayerOffline 处理玩家离线通知
func (l *PlayerSvc) PlayerOffline(ctx context.Context, msg *map[string]interface{}) (*cproto.CommonResponse, error) {
	if uid, ok := (*msg)["uid"].(string); ok {
		logrus.Infof("Player offline: %s", uid)
		// 这里可以添加玩家离线后的处理逻辑，比如清理数据、通知其他服务等
	} else {
		logrus.Warn("Received invalid player offline message")
	}

	return &cproto.CommonResponse{
		Err: 0,
	}, nil
}
