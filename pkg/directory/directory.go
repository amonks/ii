package directory

import (
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

	for app := range ports.Apps {
		row := make([]string, len(machines)+1)
		row[0] = app
		for i, machine := range machines {
			config := configs[machine]
			for _, _app := range config.Apps() {
				if app == _app {
					row[i+1] = "/" + app
					break
				}
			}
		}
		data.Rows = append(data.Rows, row)
	}

	return data, nil
}
