package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"monks.co/pkg/errlogger"
)

//go:embed mailrules.json
var rulesJSON []byte

func main() {
	if err := run(); err != nil {
		errlogger.ReportPanic(err)
		panic(err)
	}
}

func run() error {
	var rules []Rule
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return err
	}

	blockedToAddresses := map[string]struct{}{}
	blockedFromAddresses := map[string]struct{}{}

	for _, rule := range rules {
		if !rule.MarkSpam && !rule.Discard {
			continue
		}
		search := rule.Search
		if strings.HasPrefix(search, "NOT ") {
			continue
		}
		rules := strings.Split(search, " OR ")
		for _, rule := range rules {
			switch true {
			case strings.HasPrefix(rule, "from:"):
				address := strings.TrimPrefix(rule, "from:")
				blockedFromAddresses[address] = struct{}{}
			case strings.HasPrefix(rule, "to:"):
				address := strings.TrimPrefix(rule, "to:")
				blockedToAddresses[address] = struct{}{}
			default:
				log.Printf("unsupported rule '%s'", rule)
			}
		}
	}

	makeRule := func(cond, addr string) Rule {
		return Rule{
			Search:     fmt.Sprintf("%s:%s", cond, addr),
			Stop:       true,
			Discard:    true,
			Combinator: "any",
			Updated:    time.Now(),
			Created:    time.Now(),
		}
	}

	var out []Rule
	for addr := range blockedFromAddresses {
		out = append(out, makeRule("from", addr))
	}
	for addr := range blockedToAddresses {
		out = append(out, makeRule("to", addr))
	}

	bs, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bs))

	return nil
}

type Rule struct {
	FileIn             any       `json:"fileIn"`
	SkipInbox          bool      `json:"skipInbox"`
	Stop               bool      `json:"stop"`
	Search             string    `json:"search"`
	Name               string    `json:"name"`
	Combinator         string    `json:"combinator"`
	Conditions         any       `json:"conditions"`
	MarkRead           bool      `json:"markRead"`
	MarkFlagged        bool      `json:"markFlagged"`
	ShowNotification   bool      `json:"showNotification"`
	RedirectTo         any       `json:"redirectTo"`
	SnoozeUntil        any       `json:"snoozeUntil"`
	Discard            bool      `json:"discard"`
	MarkSpam           bool      `json:"markSpam"`
	Updated            time.Time `json:"updated"`
	Created            time.Time `json:"created"`
	PreviousFileInName any       `json:"previousFileInName"`
}
