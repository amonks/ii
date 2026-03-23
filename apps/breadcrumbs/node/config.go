package node

import (
	"encoding/json"
	"fmt"
)

type Config struct {
	DBPath         string         `json:"db_path"`
	Listen         string         `json:"listen"`
	Upstream       string         `json:"upstream,omitempty"`
	Capacity       int            `json:"capacity"`
	Subscriptions  []Subscription `json:"subscriptions"`
	SimplifyMethod string         `json:"simplify_method,omitempty"` // "area", "distance", "distance_floor", "multiscale"; default "distance_floor"
}

// simplifyMethod returns the configured simplify method, defaulting to distance_floor.
func (c Config) simplifyMethod() SimplifyMethod {
	if c.SimplifyMethod != "" && ValidSimplifyMethod(c.SimplifyMethod) {
		return SimplifyMethod(c.SimplifyMethod)
	}
	return MethodDistanceFloor
}

type Subscription struct {
	BBox            [4]float64 `json:"bbox"`             // [west, south, east, north]
	MinSignificance float64    `json:"min_significance"`
}

// DefaultConfig returns a root-node configuration that retains all data
// worldwide with no upstream and no eviction.
func DefaultConfig() Config {
	return Config{
		Capacity: 0,
		Subscriptions: []Subscription{
			{BBox: [4]float64{-180, -90, 180, 90}, MinSignificance: 0},
		},
	}
}

func ParseConfig(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return c, nil
}
