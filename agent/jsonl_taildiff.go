package agent

import "strings"

// JSONLTailDiff returns what should be printed when tailing a JSONL log given the
// previous snapshot and the current snapshot.
//
// Contract:
//   - Only emits *whole newly appended lines*.
//   - If the current snapshot extends the previous snapshot and ends with a
//     trailing newline, the returned diff is the newly appended bytes.
//   - If the current snapshot extends the previous snapshot but the trailing
//     line is incomplete (no final newline), returns an empty string (buffers
//     implicitly by keeping the last snapshot outside this function).
//   - If prev is empty, returns curr only if curr ends with a newline; otherwise
//     returns empty.
//   - If curr is not prefixed by prev (non-append fallback), returns curr only
//     up to the last newline (dropping any incomplete trailing line).
func JSONLTailDiff(prev, curr string) string {
	// Fast path: append-only
	if prev == "" {
		if strings.HasSuffix(curr, "\n") {
			return curr
		}
		return ""
	}

	if strings.HasPrefix(curr, prev) {
		if !strings.HasSuffix(curr, "\n") {
			return ""
		}
		return strings.TrimPrefix(curr, prev)
	}

	// Non-append fallback: print only complete lines.
	idx := strings.LastIndex(curr, "\n")
	if idx == -1 {
		return ""
	}
	return curr[:idx+1]
}
