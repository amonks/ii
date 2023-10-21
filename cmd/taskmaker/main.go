package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run() error {
	apps, err := loadApps("config/apps")
	if err != nil {
		return err
	}

	machines, err := loadMachines("config/machines")
	if err != nil {
		return err
	}

	var tasks []*task

	// add taskmaker task
	tasks = append(tasks, &task{
		Id:   "taskmaker",
		Type: "short",
		Cmd:  "go run ./cmd/taskmaker",
	})

	// add build task
	var buildDependencies []string
	for name := range apps.byName {
		buildDependencies = append(buildDependencies, "apps/"+name+"/build")
	}
	tasks = append(tasks, &task{
		Id:           "build",
		Type:         "group",
		Dependencies: buildDependencies,
	})

	// add run tasks
	for _, machine := range machines {
		var dependencies []string
		cmd := []string{
			"./bin/proxy",
			"-httpRedirectAddress=0.0.0.0:" + machine.httpPort,
			"-httpsAddress=0.0.0.0:" + machine.httpsPort,
			"-acmeConfig=acme-" + machine.name + ".toml",
		}
		if machine.mode == "dev" {
			cmd[0] = "go run ./apps/proxy"
		}
		for _, app := range apps.byMachine[machine.name] {
			cmd = append(cmd, app.name+":"+app.port)
			switch machine.mode {
			case "dev":
				dependencies = append(dependencies, "apps/"+app.name+"/dev")
			case "prod":
				dependencies = append(dependencies, "apps/"+app.name+"/start")
			default:
				return fmt.Errorf("unexpected machine mode '%s'", machine.mode)
			}
		}
		tasks = append(tasks, &task{
			Id:           machine.name,
			Type:         "long",
			Dependencies: dependencies,
			Watch:        []string{"apps/proxy/**", "apps/proxy/*"},
			Cmd:          strings.Join(cmd, " \\\n"),
		})
	}

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
	Dependencies []string `toml:"dependencies"`
	Watch        []string `toml:"watch"`
	Cmd          string   `toml:"cmd"`
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
