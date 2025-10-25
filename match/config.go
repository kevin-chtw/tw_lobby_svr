package match

import (
	"os"

	"github.com/kevin-chtw/tw_common/matchbase"
	"gopkg.in/yaml.v3"
)

type Config struct {
	*matchbase.Config
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
