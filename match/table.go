package match

import (
	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/sproto"
)

type Table struct {
	*matchbase.Table
}

func NewTable(m *Match) *Table {
	t := &Table{
		Table: matchbase.NewTable(m.Match),
	}
	t.SendAddTableReq(1, nil)
	return t
}

func (t *Table) gameResult(msg *sproto.GameResultReq) error {
	for p, s := range msg.Scores {
		t.Players[p].Score = s
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
	delete(t.Players, player.ID)
	ack := t.SendExitTableReq(player)
	if ack == nil || ack.Result != 0 {
		return false
	}
	return true
}
