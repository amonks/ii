// Package workflow executes data-driven workflows: directed graphs of shell
// commands with conditional routing and a persistent scratchpad for inter-node
// message passing.
package workflow

// Workflow is a static definition of nodes, edges, and inputs.
type Workflow struct {
	Name   string
	Start  string // Name of the start node.
	Inputs []Input
	Nodes  []Node
	Edges  []Edge
}

// Input declares a named workflow parameter.
type Input struct {
	Name     string
	Required bool
	Default  string
}

// Node is a shell command to execute.
type Node struct {
	Name    string
	Command string // Executed via bash -c.
	TTY     bool   // If true, stdin/stdout/stderr connect to the terminal.
}

// Edge connects two nodes with an optional condition.
type Edge struct {
	From      string
	To        string
	Condition string // Shell command; taken when exits 0. Empty = unconditional.
}
