package match

import (
	"context"
	"errors"
	"sync"
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
	conf        *Config
	preTable    *Table
	tables      sync.Map
	restPlayers map[string]*Player
}

func NewMatch(app pitaya.Pitaya, conf *Config) *Match {
	m := &Match{conf: conf, restPlayers: make(map[string]*Player)}
	m.Match = matchbase.NewMatch(app, conf.Config, m)
	return m
}

func (m *Match) HandleSignup(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}

	if player := m.Playermgr.Load(uid); player != nil {
		return nil, errors.New("player is in match")
	}

	player := NewPlayer(ctx, uid, m.conf.Matchid, m.conf.InitialChips)
	if err := m.addPlayer(player); err != nil {
		return nil, err
	}

	m.Playermgr.Store(player.Player)
	return m.NewStartClientAck(player.Player), nil
}

func (m *Match) addPlayer(player *Player) error {
	if m.preTable == nil {
		m.preTable = NewTable(m)
		m.tables.Store(m.preTable.ID, m.preTable)
	}
	if err := m.preTable.AddPlayer(player.Player); err != nil {
		return err
	}
	if len(m.preTable.Players) >= int(m.conf.PlayerPerTable) {
		m.preTable = nil
	}
	return nil
}

func (m *Match) HandleContinue(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}
	player, ok := m.restPlayers[uid]
	if !ok {
		return nil, errors.New("player is not in rest")
	}

	if err := m.addPlayer(player); err != nil {
		return nil, err
	}

	delete(m.restPlayers, uid)
	m.addPlayer(player)
	return m.NewStartClientAck(player.Player), nil
}

func (m *Match) HandleExitMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}
	player := m.Playermgr.Load(uid)
	if player == nil {
		return nil, errors.New("player is not in match")
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
		t, ok := m.tables.Load(p.TableId)
		if !ok {
			return errors.New("table not found")
		}
		return t.(*Table).NetChange(player, req.Online)
	} else {
		m.sendRestAck(player)
	}
	return nil
}

func (m *Match) HandleGameResult(msg proto.Message) error {
	req := msg.(*sproto.GameResultReq)
	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameResult(req)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(msg proto.Message) error {
	req := msg.(*sproto.GameOverReq)
	table, ok := m.tables.Load(req.Tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	m.tables.Delete(t.ID)
	for _, p := range t.Players {
		m.sendRestAck(p)
		p.Sub.(*Player).setMatchState(false)
		m.restPlayers[p.ID] = p.Sub.(*Player)
	}
	m.PutBackTableId(t.ID)
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

	table, ok := m.tables.Load(p.TableId)
	if !ok {
		logger.Log.Errorf("Failed to find table: %v", err)
		return
	}
	if table.(*Table).ExitTable(p) {
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
		if time.Since(p.startTime) > time.Minute*8 {
			m.sendExitMatchAck(p.Player)
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
