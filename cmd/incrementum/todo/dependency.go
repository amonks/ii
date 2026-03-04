package todo

import "time"

// Dependency represents a relationship between two todos.
type Dependency struct {
	// TodoID is the todo that has the dependency.
	TodoID string `json:"todo_id"`

	// DependsOnID is the todo that TodoID depends on.
	DependsOnID string `json:"depends_on_id"`

	// CreatedAt is when the dependency was created.
	CreatedAt time.Time `json:"created_at"`
}

// DepTreeNode represents a node in a dependency tree.
type DepTreeNode struct {
	// Todo is the todo at this node.
	Todo *Todo

	// Children are the todos that this todo depends on.
	Children []*DepTreeNode
}
