package match

import (
	"context"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
)

type Player struct {
	*matchbase.Player
	startTime time.Time
	playing   bool
}

func NewPlayer(ctx context.Context, id string, matchId int32, score int64) *Player {
	p := &Player{startTime: time.Now(), playing: true}
	p.Player = matchbase.NewPlayer(p, ctx, id, matchId, score)
	return p
}

func (p *Player) setMatchState(playing bool) {
	p.playing = playing
	p.startTime = time.Now()
}
