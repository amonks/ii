package main

var baseTasks = []*task{
	{
		Id:   "taskmaker",
		Type: "short",
		Cmd:  "go run ./cmd/taskmaker",
	},
	{
		Id:           "terraform-plan",
		Type:         "group",
		Dependencies: []string{"aws/plan"},
	},
	{
		Id:           "terraform-apply",
		Type:         "group",
		Dependencies: []string{"aws/apply"},
	},
	{
		Id:           "terraform-format",
		Type:         "group",
		Dependencies: []string{"aws/format"},
	},
}
