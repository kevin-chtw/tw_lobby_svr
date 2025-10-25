package match

import pitaya "github.com/topfreegames/pitaya/v3/pkg"

var (
	defaultMatchmgr *Matchmgr
)

// Init 初始化游戏模块
func Init(app pitaya.Pitaya) {
	defaultMatchmgr = NewMatchmgr(app)
}

func GetMatch(matchid int32) *Match {
	return defaultMatchmgr.Get(matchid)
}
