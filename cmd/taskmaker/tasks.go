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
		Dependencies: []string{"templ", "compress-statics", "zone2terraform"},
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
		Cmd:  "go run ./cmd/compressor -dir=apps/proxy/static -workers=8 -v",
	},
	{
		Id:    "zone2terraform",
		Type:  "short",
		Cmd:   "go run ./cmd/zone2terraform -dir=aws/zones -out=aws/terraform",
		Watch: []string{"aws/zones/*"},
	},
	{
		Id:           "test",
		Type:         "short",
		Dependencies: []string{"staticcheck", "go-test", "gofix", "govulncheck"},
	},
	{
		Id:   "gofix",
		Type: "short",
		Cmd:  "diff=$(go fix -diff monks.co/...); if [ -n \"$diff\" ]; then echo \"$diff\"; exit 1; fi",
	},
	{
		Id:   "staticcheck",
		Type: "short",
		Cmd:  "go tool staticcheck monks.co/...",
	},
	{
		Id:   "govulncheck",
		Type: "short",
		Cmd:  "go tool govulncheck monks.co/...",
	},
	{
		Id:   "go-test",
		Type: "short",
		Cmd:  "go test monks.co/...",
	},
	{
		Id:   "check-for-diff",
		Type: "short",
		Cmd:  "diff=$(jj diff); if [ -n \"$diff\" ]; then echo \"ERROR: working copy has changes after generate:\"; echo \"$diff\"; exit 1; fi",
	},
	{
		Id:           "ci-test",
		Type:         "short",
		Dependencies: []string{"test", "check-for-diff"},
	},
}
