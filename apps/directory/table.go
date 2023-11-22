package main

import (
	"fmt"
	"sort"

	"monks.co/pkg/config"
	"monks.co/pkg/ports"
)

type Table struct {
	Headers []string
	Rows    [][]string
}

func LoadTable() (Table, error) {
	machines, err := config.ListMachines()
	if err != nil {
		return Table{}, err
	}

	configs := make(map[string]*config.Config, len(machines))
	for _, machine := range machines {
		config, err := config.Load(machine)
		if err != nil {
			return Table{}, err
		}
		configs[machine] = config
	}

	data := Table{
		Headers: append([]string{""}, machines...),
	}

	var appNames []string
	for app := range ports.Apps {
		appNames = append(appNames, app)
	}
	sort.Strings(appNames)

	for _, app := range appNames {
		row := make([]string, len(machines)+1)
		row[0] = app
		for i, machine := range machines {
			config := configs[machine]
			for _, _app := range config.Apps() {
				if app == _app {
					row[i+1] = fmt.Sprintf("https://%s.ss.cx/%s", machine, app)
					break
				}
			}
		}
		data.Rows = append(data.Rows, row)
	}

	sort.Slice(data.Rows, func(a, b int) bool {
		return data.Rows[a][0] < data.Rows[b][0]
	})

	return data, nil
}
