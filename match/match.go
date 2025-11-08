package match

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Match struct {
	*matchbase.Match
	preTable    *matchbase.Table
	restPlayers sync.Map // string -> *matchbase.Player
}

func NewMatch(app pitaya.Pitaya, file string) *matchbase.Match {
	m := &Match{}
	m.Match = matchbase.NewMatch(app, file, m)
	return m.Match
}

func (m *Match) Tick() {
	m.checkRestPlayer()
}

func (m *Match) HandleSignup(ctx context.Context, msg proto.Message) (proto.Message, error) {
	player, err := m.ValidatePlayer(
		ctx,
		matchbase.WithCheckPlayerNotInMatch(),
		matchbase.WithAllowCreateNewPlayer(),
	)
	if err != nil {
		return nil, err
	}

	if err := m.addPlayer(player); err != nil {
		return nil, err
	}
	m.AddMatchPlayer(player)
	return m.NewStartClientAck(player), nil
}

func (m *Match) addPlayer(player *matchbase.Player) error {
	if m.preTable == nil {
		m.preTable = NewTable(m.Match)
		m.AddTable(m.preTable)
	}
	if err := m.preTable.AddPlayer(player); err != nil {
		return err
	}
	if len(m.preTable.Players) >= int(m.preTable.PlayerCount) {
		m.preTable = nil
	}
	return nil
}

func (m *Match) HandleContinue(ctx context.Context, msg proto.Message) (proto.Message, error) {
	player, err := m.ValidatePlayer(
		ctx,
	)
	if err != nil {
		return nil, err
	}

	playerValue, ok := m.restPlayers.Load(player.ID)
	if !ok {
		return nil, errors.New("player is not in rest")
	}
	player = playerValue.(*matchbase.Player)
	if err := m.addPlayer(player); err != nil {
		return nil, err
	}
	m.restPlayers.Delete(player.ID)
	return m.NewStartClientAck(player), nil
}

func (m *Match) HandleExitMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	player, err := m.ValidatePlayer(
		ctx,
	)
	if err != nil {
		return nil, err
	}

	m.exitMatch(player)
	return &cproto.ExitMatchAck{}, nil
}

func (m *Match) HandleNetState(msg proto.Message) error {
	req := msg.(*sproto.NetStateReq)
	player := m.GetMatchPlayer(req.Uid)
	if player == nil {
		return errors.New("player not found")
	}

	player.Online = req.Online
	p := player.Sub.(*Player)
	if p.playing {
		t := m.GetTable(player.TableId)
		if t == nil {
			return errors.New("table not found")
		}
		return t.Sub.(*Table).NetChange(player, req.Online)
	} else {
		m.sendRestAck(player)
	}
	return nil
}

func (m *Match) HandleGameResult(msg proto.Message) error {
	req := msg.(*sproto.GameResultReq)
	t := m.GetTable(req.Tableid)
	if t == nil {
		return errors.New("table not found")
	}
	err := t.Sub.(*Table).gameResult(req)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(msg proto.Message) error {
	req := msg.(*sproto.GameOverReq)
	table := m.GetTable(req.Tableid)
	if table == nil {
		return errors.New("table not found")
	}

	t := table.Sub.(*Table)
	for _, p := range t.Players {
		m.sendRestAck(p)
		p.Sub.(*Player).setMatchState(false)
		m.restPlayers.Store(p.ID, p)
	}
	m.DelTable(t.ID)
	return nil
}

func (m *Match) sendRestAck(restPlayer *matchbase.Player) {
	rest := &cproto.RestAck{}
	data, err := m.NewMatchAck(restPlayer.Ctx, rest)
	if err != nil {
		logger.Log.Error(err.Error())
		return
	}
	m.App.SendPushToUsers(m.App.GetServer().Type, data, []string{restPlayer.ID}, "proxy")
}

func (m *Match) exitMatch(p *matchbase.Player) {
	p.Exit = true
	if !p.Sub.(*Player).playing {
		m.DelMatchPlayer(p.ID)
		m.restPlayers.Delete(p.ID)
		return
	}

	table := m.GetTable(p.TableId)
	if table == nil {
		logger.Log.Errorf("table not find")
		return
	}

	t := table.Sub.(*Table)
	if t.ExitTable(p) {
		m.DelMatchPlayer(p.ID)
	}
}

func (m *Match) checkRestPlayer() {
	m.restPlayers.Range(func(key, value any) bool {
		p := value.(*matchbase.Player)
		player := p.Sub.(*Player)
		if time.Since(player.startTime) > time.Minute*8 {
			m.sendExitMatchAck(p)
			m.DelMatchPlayer(p.ID)
			m.restPlayers.Delete(p.ID)
		}
		return true
	})
}

func (m *Match) sendExitMatchAck(p *matchbase.Player) {
	exist := &cproto.ExitMatchAck{}
	data, err := m.NewMatchAck(p.Ctx, exist)
	if err != nil {
		logger.Log.Error(err.Error())
		return
	}
	m.App.SendPushToUsers(m.App.GetServer().Type, data, []string{p.ID}, "proxy")
}
