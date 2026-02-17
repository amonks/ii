package main

var baseTasks = []*task{
	{
		Id:   "taskmaker",
		Type: "short",
		Cmd:  "go run ./cmd/taskmaker",
	},
	// Run only checks subdirectories for taskfiles if they are
	// referenced, and then sees all (including unreferenced) tasks
	// in those taskflies.
	{
		Id:           "terraform-apply",
		Type:         "short",
		Dependencies: []string{"aws/apply"},
	},
	{
		Id:           "generate",
		Type:         "short",
		Dependencies: []string{"templ", "compress-statics"},
	},
	{
		Id:    "templ",
		Type:  "short",
		Cmd:   "go tool templ generate -path=./pkg",
		Watch: []string{"pkg/**/*.templ"},
	},
	{
		Id:   "compress-statics",
		Type: "short",
		Cmd:  "go run ./cmd/compressor -dir=apps/proxy/static -workers=8 -force -v",
	},
	{
		Id:           "test",
		Type:         "short",
		Dependencies: []string{"staticcheck", "go-test", "gofix"},
	},
	{
		Id:   "gofix",
		Type: "short",
		Cmd:  "test -z \"$(go fix -diff ./...)\"",
	},
	{
		Id:   "staticcheck",
		Type: "short",
		Cmd:  "go tool staticcheck ./...",
	},
	{
		Id:   "govulncheck",
		Type: "short",
		Cmd:  "go tool govulncheck ./...",
	},
	{
		Id:   "go-test",
		Type: "short",
		Cmd:  "go test ./...",
	},
}
