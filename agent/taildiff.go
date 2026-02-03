package agent

import (
	"strings"
)

// TranscriptTailDiff returns what should be printed when tailing a transcript
// given the previous snapshot and the current snapshot.
//
// If curr extends prev, only the appended content is returned. If prev is empty,
// the full curr snapshot is returned. If curr is not prefixed by prev (e.g. the
// transcript formatter changed), the full curr snapshot is returned.
func TranscriptTailDiff(prev, curr string) string {
	if prev == "" {
		return curr
	}
	if strings.HasPrefix(curr, prev) {
		return strings.TrimPrefix(curr, prev)
	}
	return curr
}
