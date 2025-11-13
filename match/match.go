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
	"google.golang.org/protobuf/types/known/anypb"
)

type Match struct {
	*matchbase.Match
	preTable    *Table
	restPlayers sync.Map // string -> *matchbase.Player
}

func NewMatch(app pitaya.Pitaya, file string) *matchbase.Match {
	m := &Match{}
	m.Match = matchbase.NewMatch(app, file, m)
	return m.Match
}

func (m *Match) Tick() {
	m.checkRestPlayer()
	m.checkPreTableTimeout()
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
		table, err := NewTable(m.Match)
		if err != nil {
			return err
		}
		m.preTable = table.Sub.(*Table)
		m.AddTable(m.preTable.Table)
	}
	if err := m.preTable.addPlayer(player); err != nil {
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
		// 玩家在游戏中，转发给游戏服处理断线/重连
		t := m.GetTable(player.TableId)
		if t == nil {
			return errors.New("table not found")
		}
		return t.Sub.(*Table).NetChange(player, req.Online)
	} else if req.Online {
		// 玩家在休息区重连，重发 RestAck 提示可以继续游戏
		// 不要强制退出，让玩家可以选择 Continue 或自己点 Exit
		m.sendRestAck(player)
	}
	// 离线状态下不做处理，等待超时自动清理（checkRestPlayer）
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
	if restPlayer.Bot {
		return
	}
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

func (m *Match) checkPreTableTimeout() {
	if m.preTable == nil {
		return
	}

	// 检查preTable是否已经存在超过1分钟且玩家数量不足
	if m.preTable.needBot() {
		m.requestBotForPreTable()
	}
}
func (m *Match) requestBotForPreTable() {
	var availablePlayer *matchbase.Player
	m.restPlayers.Range(func(key, value any) bool {
		p := value.(*matchbase.Player)
		player := p.Sub.(*Player)
		if player.Bot && time.Now().Unix() < player.expired {
			availablePlayer = p
			return false
		}
		return true
	})

	if availablePlayer != nil {
		if err := m.addPlayer(availablePlayer); err != nil {
			logger.Log.Errorf("Failed to add player from rest to preTable: %v", err)
		} else {
			logger.Log.Infof("Player %s from rest added to preTable", availablePlayer.ID)
			m.restPlayers.Delete(availablePlayer.ID)
			return
		}
	}

	msg := m.sendAcountReq(true, &sproto.GetBotReq{})
	if msg == nil {
		return
	}

	ack, err := msg.Ack.UnmarshalNew()
	if err != nil {
		logger.Log.Errorf("Failed to unmarshal account ack: %v", err)
		return
	}
	botAck := ack.(*sproto.GetBotAck)
	player := NewPlayer(context.Background(), botAck.Uid, m.Viper.GetInt32("matchid"), m.Viper.GetInt64("initial_chips"))
	player.Sub.(*Player).setBotInfo(botAck)
	if err := m.addPlayer(player); err != nil {
		logger.Log.Errorf("Failed to add player from rest to preTable: %v", err)
	}
}

func (m *Match) sendAcountReq(bot bool, msg proto.Message) *sproto.AccountAck {
	data, err := anypb.New(msg)
	if err != nil {
		logger.Log.Errorf("failed to create anypb: %v", err)
		return nil
	}

	req := &sproto.AccountReq{
		Bot: bot,
		Req: data,
	}
	ack := &sproto.AccountAck{}
	if err = m.App.RPC(context.Background(), "account.remote.message", ack, req); err != nil {
		logger.Log.Errorf("failed to request: %v", err)
		return nil
	}
	return ack
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
