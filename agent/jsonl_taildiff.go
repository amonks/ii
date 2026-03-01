package agent

import "strings"

// JSONLTailDiff returns what should be printed when tailing a JSONL log given the
// previous snapshot and the current snapshot.
//
// Contract:
//   - Only emits *whole newly appended lines*.
//   - If the current snapshot extends the previous snapshot and contains one or
//     more complete lines, the returned diff is the newly appended complete lines.
//   - If the current snapshot extends the previous snapshot but adds no complete
//     lines (no new trailing newline), returns an empty string (buffers
//     implicitly by keeping the last snapshot outside this function).
//   - If prev is empty, returns curr only up to the last newline (if any).
//   - If curr is not prefixed by prev (non-append fallback), returns curr only
//     up to the last newline (dropping any incomplete trailing line).
func JSONLTailDiff(prev, curr string) string {
	// Fast path: append-only
	if prev == "" {
		return lastCompleteLine(curr)
	}

	if strings.HasPrefix(curr, prev) {
		lastLine := lastCompleteLine(curr)
		base := lastCompleteLine(prev)
		if len(lastLine) <= len(base) {
			return ""
		}
		return lastLine[len(base):]
	}

	return lastCompleteLine(curr)
}

func lastCompleteLine(curr string) string {
	idx := strings.LastIndex(curr, "\n")
	if idx == -1 {
		return ""
	}
	return curr[:idx+1]
}
