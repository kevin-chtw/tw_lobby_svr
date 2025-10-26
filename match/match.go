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
	restPlayers map[string]*matchbase.Player
}

func NewMatch(app pitaya.Pitaya, conf *Config) *Match {
	m := &Match{conf: conf, restPlayers: make(map[string]*matchbase.Player)}
	m.Match = matchbase.NewMatch(app, conf.Config, m)
	m.preTable = NewTable(m)
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

	module, err := m.App.GetModule("matchingstorage")
	if err != nil {
		return nil, err
	}
	ms := module.(*storage.ETCDMatching)
	if err = ms.Put(uid, m.Conf.Matchid); err != nil {
		return nil, err
	}

	player := NewPlayer(ctx, uid, m.conf.Matchid, m.preTable.ID, m.conf.InitialChips)
	m.Playermgr.Store(player.Player)
	m.addPlayer(player.Player)
	return &cproto.SignupAck{}, nil
}

func (m *Match) addPlayer(player *matchbase.Player) {
	m.preTable.players[player.ID] = player
	if len(m.preTable.players) > int(m.conf.PlayerPerTable) {
		go m.preTable.handleStart()
		m.tables.Store(m.preTable.ID, m.preTable)
		player.Sub.(*Player).setMatchState(MatchStatePlaying)
		m.preTable = NewTable(m)
	}
}

func (m *Match) HandleSignout(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}
	player, ok := m.preTable.players[uid]
	if !ok {
		return nil, errors.New("player is not in match")
	}

	delete(m.preTable.players, uid)
	m.existMatch(player)
	return &cproto.SignoutAck{}, nil
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
	delete(m.restPlayers, uid)
	m.addPlayer(player)
	return &cproto.ContinueAck{}, nil
}

func (m *Match) HandleExistMatch(ctx context.Context, msg proto.Message) (proto.Message, error) {
	uid := m.App.GetSessionFromCtx(ctx).UID()
	if uid == "" {
		return nil, errors.New("no logged in")
	}
	player, ok := m.preTable.players[uid]
	if !ok {
		return nil, errors.New("player is not in match")
	}

	m.existMatch(player)
	return &cproto.ExitMatchAck{}, nil
}

func (m *Match) HandleNetState(msg proto.Message) error {
	req := msg.(*sproto.NetStateReq)
	player := m.Playermgr.Load(req.Uid)
	if !player.SetState(req.Online) {
		return nil
	}

	p := player.Sub.(*Player)
	switch p.matchState {
	case MatchStatePlaying:
		t, ok := m.tables.Load(p.TableId)
		if !ok {
			return errors.New("table not found")
		}
		return t.(*Table).netChange(player, req.Online)
	case MatchStateResting:
		m.sendRestAck(player)
	default:
		m.sendSignupAck(player)
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
	for _, p := range t.players {
		m.sendRestAck(p)
		p.Sub.(*Player).setMatchState(MatchStateResting)
		m.restPlayers[p.ID] = p
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

func (m *Match) sendSignupAck(restPlayer *matchbase.Player) {
	sign := &cproto.SignupAck{}
	data, err := m.NewMatchAck(restPlayer.Ctx, sign)
	if err != nil {
		logger.Log.Error(err.Error())
		return
	}
	m.App.SendPushToUsers(m.App.GetServer().Type, data, []string{restPlayer.ID}, "proxy")
}

func (m *Match) existMatch(p *matchbase.Player) {
	module, err := m.App.GetModule("matchingstorage")
	if err != nil {
		logger.Log.Errorf(err.Error())
		return
	}
	ms := module.(*storage.ETCDMatching)
	if err = ms.Remove(p.ID); err != nil {
		logger.Log.Errorf("Failed to remove player from etcd: %v", err)
	}
	p.State = 2
	matchState := p.Sub.(*Player).matchState

	if matchState == MatchStateResting {
		m.Playermgr.Delete(p.ID)
		delete(m.restPlayers, p.ID)
	}
	if matchState == MatchStatePlaying {
		table, ok := m.tables.Load(p.TableId)
		if !ok {
			logger.Log.Errorf("Failed to find table: %v", err)
			return
		}
		if table.(*Table).ExitTable(p) {
			m.Playermgr.Delete(p.ID)
		}
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
			m.sendExistMatchAck(p)
			m.Playermgr.Delete(p.ID)
			if err = ms.Remove(p.ID); err != nil {
				logger.Log.Errorf("Failed to remove player from etcd: %v", err)
				continue
			}
		}
	}
}

func (m *Match) sendExistMatchAck(p *matchbase.Player) {
	exist := &cproto.ExitMatchAck{}
	data, err := m.NewMatchAck(p.Ctx, exist)
	if err != nil {
		logger.Log.Error(err.Error())
		return
	}
	m.App.SendPushToUsers(m.App.GetServer().Type, data, []string{p.ID}, "proxy")
}
