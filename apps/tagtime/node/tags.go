package node

import "regexp"

var tagRe = regexp.MustCompile(`#(\w+)`)

// ExtractTags returns deduplicated, ordered tags from a blurb string.
// Tags are #word tokens (letters, digits, underscores).
func ExtractTags(blurb string) []string {
	matches := tagRe.FindAllStringSubmatch(blurb, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var tags []string
	for _, m := range matches {
		tag := m[1]
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

// TagRename records a time-scoped tag rename event.
type TagRename struct {
	OldName   string `json:"old_name"`
	NewName   string `json:"new_name"`
	RenamedAt int64  `json:"renamed_at"` // unix seconds
	NodeID    string `json:"node_id"`
}
