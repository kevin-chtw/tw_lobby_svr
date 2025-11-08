package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_island_svr/match"
	"github.com/kevin-chtw/tw_proto/cproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Player struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(*match.Match, context.Context, proto.Message) (proto.Message, error)
}

func NewPlayer(app pitaya.Pitaya) *Player {
	return &Player{
		app:      app,
		handlers: make(map[string]func(*match.Match, context.Context, proto.Message) (proto.Message, error)),
	}
}

func (p *Player) Init() {
	p.handlers[utils.TypeUrl(&cproto.SignupReq{})] = (*match.Match).HandleSignup
	p.handlers[utils.TypeUrl(&cproto.ContinueReq{})] = (*match.Match).HandleContinue
	p.handlers[utils.TypeUrl(&cproto.ExitMatchReq{})] = (*match.Match).HandleExitMatch
}

func (p *Player) Message(ctx context.Context, data []byte) ([]byte, error) {
	req := &cproto.MatchReq{}
	if err := utils.Unmarshal(ctx, data, req); err != nil {
		return nil, err
	}
	msg, err := req.Req.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	logger.Log.Info(msg)
	base := matchbase.GetMatch(req.Matchid)
	if base == nil {
		return nil, fmt.Errorf("match not found for ID %d", req.Matchid)
	}
	m := base.Sub.(*match.Match)
	if handler, ok := p.handlers[req.Req.TypeUrl]; ok {
		if rsp, err := handler(m, ctx, msg); err != nil {
			return nil, err
		} else {
			return base.NewMatchAck(ctx, rsp)
		}
	}
	return nil, errors.ErrUnsupported
}
