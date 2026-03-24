package node

import (
	"sort"
	"strings"

	"monks.co/pkg/color"
)

// TagTreeNode represents a node in the hierarchical tag tree.
type TagTreeNode struct {
	Name      string         `json:"name"`                // segment name, e.g. "monks.co"
	FullPath  string         `json:"full_path"`           // e.g. "coding/monks.co"
	OwnSecs   float64       `json:"own_secs"`            // time from direct pings to this exact tag
	TotalSecs float64       `json:"total_secs"`          // own + all descendants
	Color     string         `json:"color"`               // color based on full_path
	Sparkline []float64      `json:"sparkline"`           // aggregated sparkline (own + descendants)
	Children  []*TagTreeNode `json:"children,omitempty"`

	children map[string]*TagTreeNode // build-time only
}

// BuildTagTree constructs a hierarchical tree from a flat list of tag summaries.
// Tags are split on "/" to form the hierarchy. Parent nodes aggregate child time.
// Children at each level are sorted by TotalSecs descending.
func BuildTagTree(summaries []TagSummary) []*TagTreeNode {
	if len(summaries) == 0 {
		return nil
	}

	// Determine sparkline length from first non-empty sparkline.
	sparkLen := 0
	for _, s := range summaries {
		if len(s.Sparkline) > 0 {
			sparkLen = len(s.Sparkline)
			break
		}
	}

	// Build a trie of tree nodes.
	roots := make(map[string]*TagTreeNode)

	for _, s := range summaries {
		parts := strings.Split(s.Name, "/")
		var parent map[string]*TagTreeNode = roots
		var parentNode *TagTreeNode
		fullPath := ""

		for i, part := range parts {
			if i == 0 {
				fullPath = part
			} else {
				fullPath = fullPath + "/" + part
			}

			node, ok := parent[part]
			if !ok {
				node = &TagTreeNode{
					Name:      part,
					FullPath:  fullPath,
					Color:     color.Hash(fullPath),
					Sparkline: make([]float64, sparkLen),
					children:  make(map[string]*TagTreeNode),
				}
				parent[part] = node
				if parentNode != nil {
					parentNode.Children = append(parentNode.Children, node)
				}
			}

			// If this is the leaf (exact tag match), set OwnSecs and sparkline.
			if i == len(parts)-1 {
				node.OwnSecs = s.TotalSecs
				node.Color = s.Color
				if len(s.Sparkline) > 0 {
					copy(node.Sparkline, s.Sparkline)
				}
			}

			parent = node.children
			parentNode = node
		}
	}

	// Propagate totals bottom-up via post-order traversal.
	rootSlice := sortedValues(roots)
	for _, root := range rootSlice {
		propagate(root, sparkLen)
	}

	// Sort roots by TotalSecs desc.
	sort.Slice(rootSlice, func(i, j int) bool {
		if rootSlice[i].TotalSecs != rootSlice[j].TotalSecs {
			return rootSlice[i].TotalSecs > rootSlice[j].TotalSecs
		}
		return rootSlice[i].Name < rootSlice[j].Name
	})

	// Clean up build-time fields and nil out empty children.
	for _, root := range rootSlice {
		cleanup(root)
	}

	return rootSlice
}

// propagate does a post-order traversal to compute TotalSecs and aggregate sparklines.
func propagate(node *TagTreeNode, sparkLen int) {
	childTotal := 0.0
	childSparkline := make([]float64, sparkLen)

	for _, child := range node.Children {
		propagate(child, sparkLen)
		childTotal += child.TotalSecs
		for i := range childSparkline {
			if i < len(child.Sparkline) {
				childSparkline[i] += child.Sparkline[i]
			}
		}
	}

	node.TotalSecs = node.OwnSecs + childTotal
	// Aggregate sparkline: own + children.
	for i := range node.Sparkline {
		if i < len(childSparkline) {
			node.Sparkline[i] += childSparkline[i]
		}
	}

	// Sort children by TotalSecs desc.
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].TotalSecs != node.Children[j].TotalSecs {
			return node.Children[i].TotalSecs > node.Children[j].TotalSecs
		}
		return node.Children[i].Name < node.Children[j].Name
	})
}

// cleanup removes build-time fields and nils out empty children slices.
func cleanup(node *TagTreeNode) {
	node.children = nil
	if len(node.Children) == 0 {
		node.Children = nil
	}
	for _, child := range node.Children {
		cleanup(child)
	}
}

func sortedValues(m map[string]*TagTreeNode) []*TagTreeNode {
	result := make([]*TagTreeNode, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}
