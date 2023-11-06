package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"monks.co/pkg/config"
	"monks.co/pkg/ports"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	machines, err := config.ListMachines()
	if err != nil {
		return err
	}

	machineConfigs := make(map[string]*config.Config, len(machines))
	for _, machine := range machines {
		config, err := config.Load(machine)
		if err != nil {
			return err
		}
		machineConfigs[machine] = config
	}

	var tasks []*task

	// add base tasks
	tasks = append(tasks, baseTasks...)

	// add build tasks
	var buildDependencies []string
	buildDependencies = append(buildDependencies, "apps/proxy/build")
	for name := range ports.Apps {
		buildDependencies = append(buildDependencies, "apps/"+name+"/build")
	}
	sort.Strings(buildDependencies)
	tasks = append(tasks, &task{
		Id:           "build",
		Type:         "group",
		Dependencies: buildDependencies,
	})

	// add run tasks
	for machineName, machine := range machineConfigs {
		group := []string{machineName + "-proxy"}
		proxyCmd := []string{
			"./bin/proxy",
			"-machine=" + machineName,
		}
		if machine.Mode == "dev" {
			proxyCmd[0] = "go run ./apps/proxy"
		}
		for _, app := range machine.Apps() {
			switch machine.Mode {
			case "dev":
				group = append(group, "apps/"+app+"/dev")
			case "prod":
				group = append(group, "apps/"+app+"/start")
			default:
				return fmt.Errorf("unexpected machine mode '%s'", machine.Mode)
			}
		}
		sort.Strings(group)
		tasks = append(tasks, &task{
			Id:           machineName,
			Dependencies: group,
			Type:         "group",
		})
		tasks = append(tasks, &task{
			Id:    machineName + "-proxy",
			Type:  "long",
			Watch: []string{"apps/proxy/**", "apps/proxy/*"},
			Cmd:   strings.Join(proxyCmd, " "),
		})
	}

	sort.Slice(tasks, func(a, b int) bool {
		return tasks[a].Id < tasks[b].Id
	})

	bs, err := toml.Marshal(struct {
		Task []*task `toml:"task"`
	}{
		Task: tasks,
	})
	if err != nil {
		return err
	}

	os.WriteFile("tasks.toml", bs, 0x644)

	return nil
}

type task struct {
	Id           string   `toml:"id"`
	Type         string   `toml:"type"`
	Dependencies []string `toml:"dependencies,multiline,omitempty"`
	Watch        []string `toml:"watch,omitempty"`
	Cmd          string   `toml:"cmd,multiline,omitempty"`
}

type machine struct {
	name      string
	mode      string
	httpPort  string
	httpsPort string
}

func loadMachines(file string) (map[string]machine, error) {
	machinesfile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	machines := map[string]machine{}
	for _, line := range strings.Split(strings.TrimSpace(string(machinesfile)), "\n") {
		parts := strings.Split(line, "\t")
		name, mode, httpPort, httpsPort := parts[0], parts[1], parts[2], parts[3]
		machines[name] = machine{
			name:      name,
			mode:      mode,
			httpPort:  httpPort,
			httpsPort: httpsPort,
		}
	}
	return machines, nil
}

type apps struct {
	byName    map[string]*app
	byPort    map[string]*app
	byMachine map[string][]*app
	list      []*app
}

type app struct {
	port     string
	name     string
	machines []string
}

func loadApps(file string) (*apps, error) {
	appsfile, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	apps := apps{
		byName:    map[string]*app{},
		byPort:    map[string]*app{},
		byMachine: map[string][]*app{},
	}
	for _, line := range strings.Split(strings.TrimSpace(string(appsfile)), "\n") {
		parts := strings.Split(line, "\t")
		port, name, machines := parts[0], parts[1], strings.Split(parts[2], " ")
		app := &app{port, name, machines}
		apps.byName[name] = app
		apps.byPort[port] = app
		for _, machine := range machines {
			apps.byMachine[machine] = append(apps.byMachine[machine], app)
		}
		apps.list = append(apps.list, app)
	}
	return &apps, nil
}
