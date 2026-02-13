package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"tailscale.com/tsnet"
)

func main() {
	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		log.Fatal("set TS_AUTHKEY")
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
}
