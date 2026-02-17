package ids

import (
	"sort"
	"strings"
	"unicode/utf8"

	internalstrings "github.com/amonks/incrementum/internal/strings"
)

// NormalizeUniqueIDs lowercases IDs and removes duplicates or empty values.
func NormalizeUniqueIDs(ids []string) []string {
	uniqueIDs := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idLower := normalizeID(id)
		if idLower == "" {
			continue
		}
		if _, ok := seen[idLower]; ok {
			continue
		}
		seen[idLower] = struct{}{}
		uniqueIDs = append(uniqueIDs, idLower)
	}
	return uniqueIDs
}

func normalizeID(id string) string {
	for i := 0; i < len(id); i++ {
		b := id[i]
		if b >= utf8.RuneSelf || (b >= 'A' && b <= 'Z') {
			return internalstrings.NormalizeLower(id)
		}
	}
	return id
}

// UniquePrefixLengths returns the shortest unique prefix length for each ID.
func UniquePrefixLengths(ids []string) map[string]int {
	uniqueIDs := NormalizeUniqueIDs(ids)
	return UniquePrefixLengthsNormalized(uniqueIDs)
}

// UniquePrefixLengthsNormalized returns the shortest unique prefix length for each ID.
// It expects ids to already be normalized and de-duplicated.
func UniquePrefixLengthsNormalized(ids []string) map[string]int {
	lengths := make(map[string]int, len(ids))
	if len(ids) == 0 {
		return lengths
	}
	ordered := make([]string, len(ids))
	copy(ordered, ids)
	sort.Strings(ordered)
	for i, id := range ordered {
		maxPrefix := 0
		if i > 0 {
			maxPrefix = commonPrefixLength(id, ordered[i-1])
		}
		if i+1 < len(ordered) {
			nextPrefix := commonPrefixLength(id, ordered[i+1])
			if nextPrefix > maxPrefix {
				maxPrefix = nextPrefix
			}
		}
		prefixLength := min(maxPrefix+1, len(id))
		lengths[id] = prefixLength
	}

	return lengths
}

// MatchPrefix returns the matching ID for a non-empty prefix.
// The returned match preserves the original ID casing.
func MatchPrefix(ids []string, prefix string) (string, bool, bool) {
	return matchPrefix(ids, prefix, true)
}

// MatchPrefixNormalized returns the matching ID for a non-empty prefix.
// It expects ids to already be normalized and de-duplicated.
func MatchPrefixNormalized(ids []string, prefix string) (string, bool, bool) {
	return matchPrefix(ids, prefix, false)
}

func matchPrefix(ids []string, prefix string, normalizeIDs bool) (string, bool, bool) {
	needle := normalizeID(prefix)
	var match string
	for _, id := range ids {
		idKey := id
		if normalizeIDs {
			idKey = normalizeID(id)
		}
		if idKey != needle && !strings.HasPrefix(idKey, needle) {
			continue
		}
		if match != "" {
			if normalizeIDs {
				if !strings.EqualFold(match, id) {
					return "", true, true
				}
			} else if match != id {
				return "", true, true
			}
		}
		match = id
	}

	if match == "" {
		return "", false, false
	}

	return match, true, false
}

func commonPrefixLength(left, right string) int {
	limit := min(len(right), len(left))
	for i := range limit {
		if left[i] != right[i] {
			return i
		}
	}
	return limit
}
