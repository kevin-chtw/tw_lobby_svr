package match

import (
	"context"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	"github.com/kevin-chtw/tw_proto/sproto"
)

type Player struct {
	*matchbase.Player
	startTime time.Time
	playing   bool
	expired   int64
}

func NewPlayer(ctx context.Context, id string, matchId int32, score int64) *matchbase.Player {
	p := &Player{startTime: time.Now(), playing: true}
	p.Player = matchbase.NewPlayer(p, ctx, id, matchId, score)
	return p.Player
}

func (p *Player) setMatchState(playing bool) {
	p.playing = playing
	p.startTime = time.Now()
}

func (p *Player) setBotInfo(msg *sproto.GetBotAck) {
	p.Bot = true
	p.expired = msg.Expired
}
