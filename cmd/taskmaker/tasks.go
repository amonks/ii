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
		Cmd:  "cd aws/terraform && terraform init",
	},
	{
		Id:   "terraform-plan",
		Type: "short",
		Cmd:  "cd aws/terraform && terraform plan",
	},
	{
		Id:   "terraform-apply",
		Type: "short",
		Cmd:  "cd aws/terraform && terraform apply",
	},
	{
		Id:   "terraform-fmt",
		Type: "short",
		Cmd:  "cd aws/terraform && terraform fmt",
	},
}
