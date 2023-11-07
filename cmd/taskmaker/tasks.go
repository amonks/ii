package main

var baseTasks = []*task{
	{
		Id:   "taskmaker",
		Type: "short",
		Cmd:  "go run ./cmd/taskmaker",
	},
	{
		Id:   "aws-convert-zones",
		Type: "short",
		Cmd:  "cd aws && fish convert-zones.fish",
	},
	{
		Id:   "terraform-init",
		Type: "short",
		Cmd:  "source aws/.envrc && cd aws/terraform && terraform init",
	},
	{
		Id:           "terraform-plan",
		Type:         "short",
		Cmd:          "source aws/.envrc && cd aws/terraform && terraform plan",
		Dependencies: []string{"aws-convert-zones"},
	},
	{
		Id:           "terraform-apply",
		Type:         "short",
		Cmd:          "source aws/.envrc && cd aws/terraform && yes yes | terraform apply",
		Dependencies: []string{"aws-convert-zones"},
	},
	{
		Id:           "terraform-fmt",
		Type:         "short",
		Cmd:          "source aws/.envrc && cd aws/terraform && terraform fmt",
		Dependencies: []string{"aws-convert-zones"},
	},
}
