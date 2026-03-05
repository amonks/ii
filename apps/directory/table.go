package main

import (
	"fmt"
	"sort"

	"monks.co/pkg/config"
)

type Table struct {
	Headers []string
	Rows    [][]string
}

func LoadTable() (Table, error) {
	cfg, err := config.LoadApps()
	if err != nil {
		return Table{}, err
	}

	hosts := cfg.ListHosts()

	data := Table{
		Headers: append([]string{""}, hosts...),
	}

	var appNames []string
	for name := range cfg.Apps {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)

	for _, app := range appNames {
		entry := cfg.Apps[app]
		row := make([]string, len(hosts)+1)
		row[0] = app

		hostBackends := map[string]string{}
		for _, r := range entry.Routes {
			host := r.Host
			if host == "fly" {
				hostBackends[host] = fmt.Sprintf("http://monks-%s-fly-%s/", app, cfg.Defaults.Region)
			} else {
				hostBackends[host] = fmt.Sprintf("http://monks-%s-%s/", app, host)
			}
		}

		for i, host := range hosts {
			if url, ok := hostBackends[host]; ok {
				row[i+1] = url
			}
		}
		data.Rows = append(data.Rows, row)
	}

	sort.Slice(data.Rows, func(a, b int) bool {
		return data.Rows[a][0] < data.Rows[b][0]
	})

	return data, nil
}
