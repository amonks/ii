package workflow

// InteractiveWorkflow runs an interactive Claude session for design todos.
var InteractiveWorkflow = Workflow{
	Name:  "interactive",
	Start: "mark-in-progress",
	Inputs: []Input{
		{Name: "todo-id", Required: true},
	},
	Nodes: []Node{
		{
			Name:    "mark-in-progress",
			Command: `ii todo start "$(cat $SCRATCHPAD/todo-id)"`,
		},
		{
			Name:    "write-context",
			Command: `ii todo show "$(cat $SCRATCHPAD/todo-id)" > $SCRATCHPAD/todo`,
		},
		{
			Name:    "implement",
			Command: `claude`,
			TTY:     true,
		},
		{
			Name:    "check-changes",
			Command: `jj log -r @ -T empty --no-graph > $SCRATCHPAD/empty`,
		},
		{
			Name:    "test",
			Command: `go tool run test`,
		},
		{
			Name:    "commit",
			Command: `jj commit`,
			TTY:     true,
		},
		{
			Name:    "mark-finished",
			Command: `ii todo finish "$(cat $SCRATCHPAD/todo-id)"`,
		},
	},
	Edges: []Edge{
		{From: "mark-in-progress", To: "write-context"},
		{From: "write-context", To: "implement"},
		{From: "implement", To: "check-changes"},
		{From: "check-changes", To: "test", Condition: `grep -q "false" $SCRATCHPAD/empty`},
		{From: "check-changes", To: "mark-finished"},
		{From: "test", To: "implement", Condition: `[ $EXIT_CODE -ne 0 ]`},
		{From: "test", To: "commit"},
		{From: "commit", To: "implement"},
	},
}
