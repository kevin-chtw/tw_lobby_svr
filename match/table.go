package match

import (
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/sproto"
)

type Table struct {
	*matchbase.Table
	lastMatched time.Time
}

func NewTable(m *matchbase.Match) (*matchbase.Table, error) {
	t := &Table{}
	t.Table = matchbase.NewTable(m, t)
	if err := t.SendAddTableReq(1, "", nil); err != nil {
		// 创建桌子失败，回收 table ID 并返回错误
		m.PutBackTableId(t.ID)
		return nil, err
	}
	return t.Table, nil
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	for p, s := range msg.Scores {
		if player, ok := t.Players[p]; ok {
			player.Score = s
		}
	}
	return nil
}

func (t *Table) ExitTable(player *matchbase.Player) bool {
	// 先向游戏服发送退出请求，成功后再删除本地状态，避免状态不一致
	if err := t.SendExitTableReq(player); err != nil {
		return false
	}
	delete(t.Players, player.ID)
	return true
}

func (t *Table) addPlayer(player *matchbase.Player) error {
	if err := t.Table.AddPlayer(player); err != nil {
		return err
	}
	t.lastMatched = time.Now()
	return nil
}

func (t *Table) needBot() bool {
	if !t.Match.Viper.GetBool("allow_bots") {
		return false
	}
	return time.Since(t.lastMatched) > 3*time.Second && len(t.Players) < int(t.PlayerCount)
}
