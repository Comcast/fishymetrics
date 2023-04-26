package config

import (
	"sync"
	"time"
)

type Config struct {
	OOBScheme  string
	OOBTimeout time.Duration
	User       string
	Pass       string
}

var (
	config *Config
	once   sync.Once
)

func NewConfig(c *Config) {
	once.Do(func() {
		if c != nil {
			config = c
		} else {
			config = &Config{}
		}
	})
}

func GetConfig() *Config {
	if config != nil {
		return config
	}

	NewConfig(nil)
	return config
}
