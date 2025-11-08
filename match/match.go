package match

import (
	"context"
	"errors"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_common/storage"
	"github.com/kevin-chtw/tw_proto/cproto"
	"github.com/kevin-chtw/tw_proto/sproto"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	"google.golang.org/protobuf/proto"
)

type Match struct {
	*matchbase.Match
	preTable    *matchbase.Table
	restPlayers map[string]*matchbase.Player
}

func NewMatch(app pitaya.Pitaya, file string) *matchbase.Match {
	m := &Match{restPlayers: make(map[string]*matchbase.Player)}
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
	if len(m.preTable.Players) >= m.Viper.GetInt("player_per_table") {
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

	player, ok := m.restPlayers[player.ID]
	if !ok {
		return nil, errors.New("player is not in rest")
	}

	if err := m.addPlayer(player); err != nil {
		return nil, err
	}

	delete(m.restPlayers, player.ID)
	m.addPlayer(player)
	return m.NewStartClientAck(player), nil
}

func (m *Match) HandleExitMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	player, err := m.ValidatePlayer(
		ctx,
		matchbase.WithCheckPlayerNotInMatch(),
	)
	if err != nil {
		return nil, err
	}

	m.exitMatch(player)
	return &cproto.ExitMatchAck{}, nil
}

func (m *Match) HandleNetState(msg proto.Message) error {
	req := msg.(*sproto.NetStateReq)
	player := m.Playermgr.Load(req.Uid)
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
		m.restPlayers[p.ID] = p
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
	module, err := m.App.GetModule("matchingstorage")
	if err != nil {
		logger.Log.Errorf(err.Error())
		return
	}
	ms := module.(*storage.ETCDMatching)
	if err = ms.Remove(p.ID); err != nil {
		logger.Log.Errorf("Failed to remove player from etcd: %v", err)
	}
	p.Exit = true
	if !p.Sub.(*Player).playing {
		m.Playermgr.Delete(p.ID)
		delete(m.restPlayers, p.ID)
		return
	}

	table := m.GetTable(p.TableId)
	if table == nil {
		logger.Log.Errorf("table not find")
		return
	}

	t := table.Sub.(*Table)
	if t.ExitTable(p) {
		m.Playermgr.Delete(p.ID)
	}
}

func (m *Match) checkRestPlayer() {
	module, err := m.App.GetModule("matchingstorage")
	if err != nil {
		logger.Log.Errorf(err.Error())
		return
	}
	ms := module.(*storage.ETCDMatching)
	for _, p := range m.restPlayers {
		player := p.Sub.(*Player)
		if time.Since(player.startTime) > time.Minute*8 {
			m.sendExitMatchAck(p)
			m.Playermgr.Delete(p.ID)
			if err = ms.Remove(p.ID); err != nil {
				logger.Log.Errorf("Failed to remove player from etcd: %v", err)
				continue
			}
		}
	}
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
