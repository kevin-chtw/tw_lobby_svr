package match

import (
	"path/filepath"
	"time"

	"github.com/kevin-chtw/tw_common/matchbase"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
)

// Matchmgr 管理玩家
type Matchmgr struct {
	*matchbase.Matchmgr
}

// NewMatchmgr 创建玩家管理器
func NewMatchmgr(app pitaya.Pitaya) *Matchmgr {
	matchmgr := &Matchmgr{
		Matchmgr: matchbase.NewMatchmgr(app),
	}
	matchmgr.LoadMatchs()
	go matchmgr.checkRestPlayer()
	return matchmgr
}

func (m *Matchmgr) checkRestPlayer() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		for _, match := range m.Matchs {
			match.Sub.(*Match).checkRestPlayer()
		}
	}
}

func (m *Matchmgr) LoadMatchs() error {
	// 获取所有比赛配置文件
	files, err := filepath.Glob(filepath.Join("etc", m.App.GetServer().Type, "*.yaml"))
	if err != nil {
		return err
	}

	// 加载每个配置文件
	for _, file := range files {
		config, err := LoadConfig(file)
		if err != nil || config == nil {
			logger.Log.Error(err.Error())
			continue
		}
		logger.Log.Infof("加载比赛配置: %s", file)
		match := NewMatch(m.App, config)
		m.Matchmgr.Add(match.Match)
	}

	return nil
}

func (m *Matchmgr) Get(matchId int32) *Match {
	match := m.Matchmgr.Get(matchId)
	if match == nil {
		return nil
	}
	return match.Sub.(*Match)
}
