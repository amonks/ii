package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

const capPrefix = "monks.co/cap/"

func main() {
	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		log.Fatal("set TS_AUTHKEY (must be an auth key with the right tags, not an API key)")
	}

	s := &tsnet.Server{
		Hostname:  "monks-captest",
		Ephemeral: true,
		AuthKey:   authKey,
	}
	if _, err := s.Up(context.Background()); err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	lc, err := s.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	st, err := lc.Status(context.Background())
	if err != nil {
		log.Fatal("Status: ", err)
	}
	fmt.Printf("node: %s\n", st.Self.HostName)
	fmt.Printf("tags: %v\n\n", st.Self.Tags)

	rules, err := lc.DebugPacketFilterRules(context.Background())
	if err != nil {
		log.Fatal("DebugPacketFilterRules: ", err)
	}

	fmt.Printf("got %d filter rules\n\n", len(rules))
	for i, rule := range rules {
		if len(rule.CapGrant) == 0 {
			continue
		}
		fmt.Printf("--- rule %d ---\n", i)
		fmt.Printf("SrcIPs: %v\n", rule.SrcIPs)
		for j, cg := range rule.CapGrant {
			fmt.Printf("  CapGrant[%d]:\n", j)
			fmt.Printf("    Dsts: %v\n", cg.Dsts)
			for cap, vals := range cg.CapMap {
				raw, _ := json.Marshal(vals)
				fmt.Printf("    Cap: %s = %s\n", cap, raw)
			}
		}
		fmt.Println()
	}

	// Run the same AnonCaps extraction the proxy uses
	caps := anonCaps(rules)
	fmt.Printf("--- AnonCaps result ---\n")
	if len(caps) == 0 {
		fmt.Println("(empty — no SrcIPs=[\"*\"] rules with CapGrant)")
	}
	for cap, vals := range caps {
		raw, _ := json.Marshal(vals)
		fmt.Printf("  %s = %s\n", cap, raw)
	}

	// Run the same route extraction the proxy uses
	routes := routesFromCaps(caps)
	fmt.Printf("\n--- Routes ---\n")
	if len(routes) == 0 {
		fmt.Println("(empty)")
	}
	for path, backend := range routes {
		fmt.Printf("  /%s/ -> %s\n", path, backend)
	}
}

func anonCaps(rules []tailcfg.FilterRule) tailcfg.PeerCapMap {
	caps := make(tailcfg.PeerCapMap)
	for _, rule := range rules {
		if !srcIPsContainsStar(rule.SrcIPs) {
			continue
		}
		for _, grant := range rule.CapGrant {
			for cap, vals := range grant.CapMap {
				caps[cap] = append(caps[cap], vals...)
			}
		}
	}
	return caps
}

func srcIPsContainsStar(srcIPs []string) bool {
	for _, ip := range srcIPs {
		if ip == "*" {
			return true
		}
	}
	return false
}

func routesFromCaps(caps tailcfg.PeerCapMap) map[string]string {
	routes := map[string]string{}
	for cap, vals := range caps {
		capStr := string(cap)
		if !strings.HasPrefix(capStr, capPrefix) {
			continue
		}
		raw, err := json.Marshal(vals)
		if err != nil {
			fmt.Printf("  warning: failed to marshal cap %s: %v\n", cap, err)
			continue
		}
		var entries []struct {
			Path    string `json:"path"`
			Backend string `json:"backend"`
		}
		if err := json.Unmarshal(raw, &entries); err != nil {
			fmt.Printf("  warning: failed to parse cap %s: %v\n    raw: %s\n", cap, err, raw)
			continue
		}
		for _, e := range entries {
			if e.Path != "" && e.Backend != "" {
				routes[e.Path] = e.Backend
			}
		}
	}
	return routes
}
