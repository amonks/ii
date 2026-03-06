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
		Dependencies: []string{"taskmaker", "templ", "compress-statics", "zone2terraform", "gofix"},
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
		Dependencies: []string{"staticcheck", "go-test", "govulncheck", "publish-validate"},
	},
	{
		Id:   "publish-validate",
		Type: "short",
		Cmd:  "go run ./cmd/publish -validate",
	},
	{
		Id:   "gofix",
		Type: "short",
		Cmd:  "echo \"=== gofix diag ===\"; go version; echo \"GOARCH=$GOARCH GOOS=$GOOS\"; echo \"GOMODCACHE=$GOMODCACHE GOCACHE=$GOCACHE GOWORK=$GOWORK\"; echo \"PWD=$PWD\"; echo \"=== go fix -diff ===\"; go fix -diff monks.co/... | tee /dev/stderr | head -100; echo \"=== applying ===\"; go fix monks.co/...",
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
