package node

import "encoding/json"

// Config holds the configuration for a tagtime node.
type Config struct {
	DBPath            string `json:"db_path"`
	Listen            string `json:"listen,omitempty"`
	Upstream          string `json:"upstream,omitempty"`
	NodeID            string `json:"node_id"`
	DefaultSeed       uint64 `json:"default_seed"`
	DefaultPeriodSecs int64  `json:"default_period_secs"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		DefaultSeed:       11193462,
		DefaultPeriodSecs: 2700, // 45 minutes
	}
}

// ParseConfig parses a JSON config.
func ParseConfig(data []byte) (Config, error) {
	c := DefaultConfig()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}
