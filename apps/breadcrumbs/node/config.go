package node

import (
	"encoding/json"
	"fmt"
)

type Config struct {
	DBPath        string         `json:"db_path"`
	Listen        string         `json:"listen"`
	Upstream      string         `json:"upstream,omitempty"`
	Capacity      int            `json:"capacity"`
	Subscriptions []Subscription `json:"subscriptions"`
}

type Subscription struct {
	BBox            [4]float64 `json:"bbox"`             // [west, south, east, north]
	MinSignificance float64    `json:"min_significance"`
}

func ParseConfig(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return c, nil
}
