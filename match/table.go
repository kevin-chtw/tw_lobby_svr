package match

import (
	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/sproto"
)

type Table struct {
	*matchbase.Table
	players map[string]*matchbase.Player
}

func NewTable(m *Match) *Table {
	return &Table{
		Table:   matchbase.NewTable(m.Match),
		players: make(map[string]*matchbase.Player),
	}
}

func (t *Table) handleStart() {
	t.SendAddTableReq(t.Match.Conf.ScoreBase, 1, t.Match.Conf.GameType, nil)
	seat := int32(0)
	for _, p := range t.players {
		t.SendAddPlayer(p, seat)
		t.SendStartClient(p)
		p.Sub.(*Player).setMatchState(MatchStatePlaying)
		seat++
	}
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	for _, p := range msg.Players {
		if player, ok := t.players[p.Playerid]; ok {
			player.Score = p.Score
		}
	}
	return nil
}

func (t *Table) netChange(player *matchbase.Player, online bool) error {
	t.SendNetState(player, online)
	if online {
		t.SendStartClient(player)
	}
	return nil
}

func (t *Table) ExitTable(player *matchbase.Player) bool {
	ack := t.SendExitTableReq(player)
	if ack == nil || ack.Result != 0 {
		return false
	}
	return true
}
