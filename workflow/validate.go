package workflow

import "fmt"

// validate checks that a workflow definition is well-formed.
func validate(wf Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if wf.Start == "" {
		return fmt.Errorf("start node is required")
	}
	if len(wf.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}

	nodeNames := make(map[string]bool, len(wf.Nodes))
	for _, n := range wf.Nodes {
		if n.Name == "" {
			return fmt.Errorf("node name is required")
		}
		if nodeNames[n.Name] {
			return fmt.Errorf("duplicate node name: %s", n.Name)
		}
		nodeNames[n.Name] = true
	}

	if !nodeNames[wf.Start] {
		return fmt.Errorf("start node %q not found in nodes", wf.Start)
	}

	for _, e := range wf.Edges {
		if !nodeNames[e.From] {
			return fmt.Errorf("edge references unknown source node: %s", e.From)
		}
		if !nodeNames[e.To] {
			return fmt.Errorf("edge references unknown destination node: %s", e.To)
		}
	}

	return nil
}
