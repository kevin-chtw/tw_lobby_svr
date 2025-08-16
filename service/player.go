package service

import (
	"context"

	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/session"
)

type Player struct {
	component.Base
	app         pitaya.Pitaya
	sessionPool session.SessionPool
}

func NewPlayer(app pitaya.Pitaya, sessionPool session.SessionPool) *Player {
	return &Player{
		app:         app,
		sessionPool: sessionPool,
	}
}

func (l *Player) Message(ctx context.Context, req *cproto.LobbyReq) (*cproto.LobbyAck, error) {
	logrus.Debugf("PlayerMsg: %v", req)

	if req.LoginReq != nil {
		return l.handleLogin(ctx, req.LoginReq)
	}

	if req.RegisterReq != nil {
		return l.handleRegister(ctx, req.RegisterReq)
	}

	return &cproto.LobbyAck{}, nil
}

func (l *Player) handleLogin(ctx context.Context, req *cproto.LoginReq) (*cproto.LobbyAck, error) {
	rsp := &sproto.GetPlayerAck{}
	if err := l.app.RPC(ctx, "db.player.get", rsp, &sproto.GetPlayerReq{
		Account:  req.Account,
		Password: req.Password,
	}); err != nil {
		return nil, err
	}

	if old := l.sessionPool.GetSessionByUID(rsp.Userid); old != nil {
		old.Kick(context.Background())
	}

	s := l.app.GetSessionFromCtx(ctx)
	if err := s.Bind(ctx, rsp.Userid); err != nil {
		return nil, err
	}
	// 返回登录成功响应
	return &cproto.LobbyAck{
		LoginAck: &cproto.LoginAck{
			Serverid: l.app.GetServerID(),
			Userid:   rsp.Userid,
		},
	}, nil
}

func (l *Player) createAccount(ctx context.Context, account, password string) (string, error) {
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

func (l *Player) handleRegister(ctx context.Context, req *cproto.RegisterReq) (*cproto.LobbyAck, error) {
	s := l.app.GetSessionFromCtx(ctx)

	// 创建新账号
	userID, err := l.createAccount(ctx, req.Account, req.Password)
	if err != nil {
		return nil, err
	}

	// 绑定用户会话
	if err := s.Bind(ctx, userID); err != nil {
		return nil, err
	}

	// 返回注册成功响应
	return &cproto.LobbyAck{
		RegisterAck: &cproto.RegisterAck{
			Serverid: l.app.GetServerID(),
			Userid:   userID,
		},
	}, nil
}

// PlayerOffline 处理玩家离线通知
func (l *Player) PlayerOffline(ctx context.Context, msg *map[string]interface{}) (*cproto.LobbyAck, error) {
	if uid, ok := (*msg)["uid"].(string); ok {
		logrus.Infof("Player offline: %s", uid)
		// 这里可以添加玩家离线后的处理逻辑，比如清理数据、通知其他服务等
	} else {
		logrus.Warn("Received invalid player offline message")
	}

	return &cproto.LobbyAck{}, nil
}
