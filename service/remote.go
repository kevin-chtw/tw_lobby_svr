package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kevin-chtw/tw_common/utils"
	"github.com/kevin-chtw/tw_island_svr/match"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Game struct {
	component.Base
	app      pitaya.Pitaya
	handlers map[string]func(*match.Match, proto.Message) error
}

func NewRemote(app pitaya.Pitaya) *Game {
	return &Game{
		app:      app,
		handlers: make(map[string]func(*match.Match, proto.Message) error),
	}
}

func (g *Game) Init() {
	g.handlers[utils.TypeUrl(&sproto.NetStateReq{})] = (*match.Match).HandleNetState
	g.handlers[utils.TypeUrl(&sproto.GameResultReq{})] = (*match.Match).HandleGameResult
	g.handlers[utils.TypeUrl(&sproto.GameOverReq{})] = (*match.Match).HandleGameOver
}

func (g *Game) Message(ctx context.Context, msg *sproto.MatchReq) (*sproto.MatchAck, error) {
	if msg == nil {
		return nil, errors.New("nil request: MatchReq cannot be nil")
	}

	logger.Log.Info(msg.String(), msg.Req.TypeUrl)

	match := match.GetMatch(msg.Matchid)
	if match == nil {
		return nil, fmt.Errorf("match not found for ID %d", msg.Matchid)
	}

	req, err := msg.Req.UnmarshalNew()
	if err != nil {
		return nil, err
	}

	if handler, ok := g.handlers[msg.Req.TypeUrl]; ok {
		if err := handler(match, req); err != nil {
			return nil, err
		} else {
			return &sproto.MatchAck{}, nil
		}
	}
	return nil, errors.New("invalid request type")
}
