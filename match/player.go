package match

import (
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
)

type RestPlayer struct {
	*matchbase.Player
	startTime time.Time
}

func NewRestPlayer(player *matchbase.Player) *RestPlayer {
	return &RestPlayer{
		Player:    player,
		startTime: time.Now(),
	}
}
