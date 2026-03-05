package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"bytes"

	"github.com/pelletier/go-toml/v2"
	"monks.co/run/taskfile"
	"monks.co/pkg/config"
	"monks.co/pkg/env"
)

func main() {
	if err := start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func start() error {
	root := env.InMonksRoot()

	// generate go.mod and go.work files
	if err := generateModuleFiles(root); err != nil {
		return fmt.Errorf("generating module files: %w", err)
	}

	var tasks []*task

	// add base tasks
	tasks = append(tasks, baseTasks...)

	// build app task sets for each machine
	appTasks, moduleGenerateDeps, err := buildAppTasks()
	if err != nil {
		return err
	}
	tasks = append(tasks, appTasks...)

	// find generate task and add discovered generate dependencies
	var generate *task
	for _, task := range tasks {
		if task.Id == "generate" {
			generate = task
			break
		}
	}
	generate.Dependencies = append(generate.Dependencies, moduleGenerateDeps...)

	// also add build-task dependencies (e.g. templ, build-css) as generators
	generators, err := findGenerateTaskIDs(tasks)
	if err != nil {
		return fmt.Errorf("finding generator tasks: %w", err)
	}
	generate.Dependencies = append(generate.Dependencies, generators...)

	// deduplicate
	slices.Sort(generate.Dependencies)
	generate.Dependencies = slices.Compact(generate.Dependencies)

	// add discovered test tasks from apps to top-level test task
	redundant, err := loadRedundantCommands(root)
	if err != nil {
		return fmt.Errorf("loading redundant commands: %w", err)
	}
	appTests, err := findAppTestTasks(root, redundant)
	if err != nil {
		return fmt.Errorf("finding app test tasks: %w", err)
	}
	if len(appTests) > 0 {
		var testTask *task
		for _, t := range tasks {
			if t.Id == "test" {
				testTask = t
				break
			}
		}
		if testTask != nil {
			testTask.Dependencies = append(testTask.Dependencies, appTests...)
		}
	}

	if err := writeTasks("tasks.toml", tasks); err != nil {
		return fmt.Errorf("writing finished tasks.toml: %w", err)
	}

	if err := generateFlyConfigs(); err != nil {
		return fmt.Errorf("generating fly configs: %w", err)
	}

	return nil
}

func buildAppTasks() ([]*task, []string, error) {
	root := env.InMonksRoot()

	// discover all modules with tasks.toml files
	moduleTasks, err := discoverModuleTasks(root)
	if err != nil {
		return nil, nil, fmt.Errorf("discovering module tasks: %w", err)
	}

	// collect modules that have build or generate tasks
	var buildDeps, generateDeps []string
	for dir, ids := range moduleTasks {
		if slices.Contains(ids, "build") {
			buildDeps = append(buildDeps, dir+"/build")
		}
		if slices.Contains(ids, "generate") {
			generateDeps = append(generateDeps, dir+"/generate")
		}
	}
	sort.Strings(buildDeps)
	sort.Strings(generateDeps)

	var tasks []*task
	tasks = []*task{{
		Id:           "build",
		Type:         "short",
		Dependencies: buildDeps,
	}}

	// add run tasks for dev machines only
	machineConfigs, err := getMachineConfigs()
	if err != nil {
		return nil, nil, err
	}
	for machineName, machine := range machineConfigs {
		if machine.Mode != "dev" {
			continue
		}

		machineStart := &task{
			Id:   machineName,
			Type: "long",
		}

		for _, app := range machine.Apps() {
			machineStart.Dependencies = append(machineStart.Dependencies,
				"apps/"+app+"/dev")
		}

		sort.Strings(machineStart.Dependencies)

		tasks = append(tasks, machineStart)
	}

	return tasks, generateDeps, nil
}

func getMachineConfigs() (map[string]*config.Config, error) {
	machines, err := config.ListMachines()
	if err != nil {
		return nil, fmt.Errorf("listing machines: %w", err)
	}

	machineConfigs := make(map[string]*config.Config, len(machines))
	for _, machine := range machines {
		config, err := config.Load(machine)
		if err != nil {
			return nil, fmt.Errorf("loading config for '%s': %w", machine, err)
		}
		machineConfigs[machine] = config
	}

	return machineConfigs, nil
}

// Find the "generate" tasks from the apps. A generate task is here defined as
// the dependency of an app's build task.
func findGenerateTaskIDs(tasks []*task) ([]string, error) {
	// We want to discover the subtasks from apps. We'll do that by writing
	// the taskfile and asking Run to load it.
	if err := writeTasks("tasks.toml", tasks); err != nil {
		return nil, err
	}
	loaded, err := taskfile.Load(".")
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, taskID := range loaded.IDs() {
		if !strings.HasPrefix(taskID, "apps/") {
			continue
		}
		if !strings.HasSuffix(taskID, "/build") {
			continue
		}

		ids = append(ids, loaded.Get(taskID).Metadata().Dependencies...)
	}

	return ids, nil
}

func writeTasks(filename string, tasks []*task) error {
	sort.Slice(tasks, func(a, b int) bool {
		return tasks[a].Id < tasks[b].Id
	})
	for _, t := range tasks {
		sort.Strings(t.Dependencies)
	}

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	enc.SetIndentTables(true)

	if err := enc.Encode(struct {
		Task []*task `toml:"task"`
	}{Task: tasks}); err != nil {
		return fmt.Errorf("marshalling taskflie: %w", err)
	}

	bs := append([]byte("# Code generated by taskmaker - DO NOT EDIT.\n\n"), buf.Bytes()...)

	if err := os.WriteFile(filename, bs, 0644); err != nil {
		return err
	}

	return nil
}

type task struct {
	Id           string   `toml:"id"`
	Type         string   `toml:"type"`
	Dependencies []string `toml:"dependencies,multiline,omitempty"`
	Watch        []string `toml:"watch,omitempty"`
	Cmd          string   `toml:"cmd,multiline,omitempty"`
}
