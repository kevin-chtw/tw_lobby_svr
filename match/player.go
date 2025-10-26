package match

import (
	"context"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
)

const (
	MatchStateMatching = 0
	MatchStatePlaying  = 1
	MatchStateResting  = 2
)

type Player struct {
	*matchbase.Player
	startTime  time.Time
	matchState int32
}

func NewPlayer(ctx context.Context, id string, matchId, tableId int32, score int64) *Player {
	p := &Player{startTime: time.Now(), matchState: MatchStateMatching}
	p.Player = matchbase.NewPlayer(p, ctx, id, matchId, tableId, score)
	return p
}

func (p *Player) setMatchState(state int32) {
	p.matchState = state
	p.startTime = time.Now()
}
