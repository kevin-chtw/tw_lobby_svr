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
	restPlayers map[string]*RestPlayer
}

func NewMatch(app pitaya.Pitaya, conf *Config) *Match {
	m := &Match{conf: conf, restPlayers: make(map[string]*RestPlayer)}
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
	if err = ms.Put(uid); err != nil {
		return nil, err
	}

	player := matchbase.NewPlayer(ctx, uid, m.conf.Matchid, m.preTable.ID, m.conf.InitialChips)
	m.Playermgr.Store(player)
	m.addPlayer(player)
	return &cproto.SignupAck{}, nil
}

func (m *Match) addPlayer(player *matchbase.Player) {
	m.preTable.players[player.ID] = player
	if len(m.preTable.players) > int(m.conf.PlayerPerTable) {
		go m.preTable.handleStart()
		m.tables.Store(m.preTable.ID, m.preTable)
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
	m.addPlayer(player.Player)
	return &cproto.ContinueAck{}, nil

}

func (m *Match) HandleGameResult(tableid int32, msg proto.Message) error {
	ack := msg.(*sproto.GameResultAck)

	table, ok := m.tables.Load(tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	err := t.gameResult(ack)
	if err != nil {
		logger.Log.Errorf("Failed to handle game result: %v", err)
	}
	return err
}

func (m *Match) HandleGameOver(tableid int32, msg proto.Message) error {
	table, ok := m.tables.Load(tableid)
	if !ok {
		return errors.New("table not found")
	}

	t := table.(*Table)
	m.tables.Delete(t.ID)
	for _, p := range t.players {
		restPlayer := NewRestPlayer(p)
		m.sendRestAck(restPlayer)
		m.restPlayers[p.ID] = restPlayer
	}
	m.PutBackTableId(t.ID)
	return nil
}

func (m *Match) sendRestAck(restPlayer *RestPlayer) {
	rest := &cproto.RestAck{}
	data, err := m.NewMatchAck(restPlayer.Ctx, rest)
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
	// TODO 在比赛中的只能标记
	m.Playermgr.Delete(p.ID)
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
			m.sendExistMatchAck(p)
			m.Playermgr.Delete(p.ID)
			if err = ms.Remove(p.ID); err != nil {
				logger.Log.Errorf("Failed to remove player from etcd: %v", err)
				continue
			}
		}
	}
}

func (m *Match) sendExistMatchAck(p *RestPlayer) {
	exist := &cproto.ExistMatchAck{}
	data, err := m.NewMatchAck(p.Ctx, exist)
	if err != nil {
		logger.Log.Error(err.Error())
		return
	}
	m.App.SendPushToUsers(m.App.GetServer().Type, data, []string{p.ID}, "proxy")
}
