package config

import (
	"github.com/empirefox/confy/xps"
)

func LoadFromXpsWithEnv() (*Config, error) {
	config := new(Config)
	err := crypt.LoadConfig(config, nil)
	if err != nil {
		return nil, err
	}
	return config, nil
}
