package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	StoragePath string    `toml:"storage_path"`
	Services    []Service `toml:"services"`
}

type Service struct {
	Port     int    `toml:"port"`
	Protocol string `toml:"protocol"`
	IsAdmin  bool   `toml:"is_admin"`
	ACME     *ACME  `toml:"acme"`
}

type ACME struct {
	Strategies []ACMEStrategy `toml:"strategies"`
	Domains    []string       `toml:"domains"`
	Production bool           `toml:"production"`
}

type ACMEStrategy struct {
	Strategy     string `toml:"strategy"`
	ExternalPort int    `toml:"external_port"`
	InternalPort int    `toml:"internal_port"`
}

var config *Config

func init() {
	env := os.Getenv("MONKSCO_ENV")
	if env == "" {
		panic("MONKSCO_ENV not set")
	}

	configPath := fmt.Sprintf("env/%s.toml", env)
	c, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	if err := toml.Unmarshal(c, &config); err != nil {
		panic(err)
	}
}

func Get() Config {
	if config == nil {
		panic("no config")
	}

	return *config
}
