package workflow

// Registry maps workflow names to their definitions.
var Registry = map[string]Workflow{
	"job":         JobWorkflow,
	"merge":       MergeWorkflow,
	"habit":       HabitWorkflow,
	"interactive": InteractiveWorkflow,
}
