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

func (t *Table) ExitTable(player *matchbase.Player) bool {
	delete(t.Players, player.ID)
	if err := t.SendExitTableReq(player); err != nil {
		return false
	}
	return true
}
