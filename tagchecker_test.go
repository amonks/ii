package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

type lowercaseMatch struct {
	File string
	Base string
	Line string
}

var lowercaseRegexCache sync.Map

func TestTagCoverage(t *testing.T) {
	tagMap := buildTagMap()
	allowed := loadAllowedAllCaps(t)

	summaryRoot := path.Join(notesRoot, summaryPhysicalDir)
	t.Run("summaries", func(t *testing.T) {
		issues := collectDirIssues(t, summaryRoot, tagMap, summaryRoot+"/", allowed)
		if len(issues) > 0 {
			t.Fatalf("found non-tag capitalized strings in summaries:\n%s", formatIssues(issues, false, true))
		}
	})

	t.Run("sessions", func(t *testing.T) {
		sessionPaths := loadImportedSessions(t)
		if len(sessionPaths) == 0 {
			t.Log("no imported sessions listed; skipping session tag check")
			return
		}

		issues := make(map[string]map[string]int)
		for _, rel := range sessionPaths {
			fullPath := path.Join("notes", rel)
			fileIssues := collectFileIssues(t, fullPath, tagMap, allowed)
			if len(fileIssues) > 0 {
				issues[rel] = fileIssues
			}
		}

		if len(issues) > 0 {
			t.Fatalf("found non-tag capitalized strings in sessions:\n%s", formatIssues(issues, true, false))
		}
	})
}

func TestTagMapIncludesExistingTags(t *testing.T) {
	tagMap := buildTagMap()
	required := []string{"PUDDING", "ZION MD"}
	for _, tag := range required {
		if _, ok := tagMap[tag]; !ok {
			t.Fatalf("tagMap missing expected tag %s", tag)
		}
	}
}

func TestTagNameUniqueness(t *testing.T) {
	tagCategories := summaryTagsByCategory(t)
	var conflicts []string
	for tag, cats := range tagCategories {
		if len(cats) > 1 {
			sort.Strings(cats)
			conflicts = append(conflicts, fmt.Sprintf("%s (%s)", tag, strings.Join(cats, ", ")))
		}
	}

	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		t.Fatalf("tags must be unique across categories:\n%s", strings.Join(conflicts, "\n"))
	}
}

func TestSummaryTagsAppearInSessions(t *testing.T) {
	tagCategories := summaryTagsByCategory(t)
	sessionTags := collectSessionTags(t)

	var missing []string
	for tag := range tagCategories {
		canonical := canonicalTagName(tag)
		if canonical == "" {
			continue
		}
		if _, ok := sessionTags[canonical]; !ok {
			missing = append(missing, canonical)
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("summary tags missing from session notes:\n%s", strings.Join(missing, "\n"))
	}
}

func TestTagsNotLowercase(t *testing.T) {
	tagMap := buildTagMap()
	allowed := loadAllowedAllCaps(t)
	combinedIssues := make(map[string][]lowercaseMatch)

	summaryRoot := path.Join(notesRoot, summaryPhysicalDir)
	t.Run("summaries", func(t *testing.T) {
		issues := collectLowercaseIssuesInDir(t, summaryRoot, summaryRoot+"/", tagMap, allowed)
		if len(issues) > 0 {
			mergeLowercaseIssues(combinedIssues, issues)
		}
	})

	t.Run("sessions", func(t *testing.T) {
		sessionPaths := loadImportedSessions(t)
		if len(sessionPaths) == 0 {
			t.Log("no imported sessions listed; skipping session lowercase tag check")
			return
		}

		for _, rel := range sessionPaths {
			fullPath := path.Join("notes", rel)
			fileIssues := collectLowercaseIssuesInFile(t, fullPath, rel, tagMap, allowed)
			if len(fileIssues) > 0 {
				mergeLowercaseIssues(combinedIssues, fileIssues)
			}
		}
	})

	if len(combinedIssues) > 0 {
		t.Fatalf("found lowercase tag references:\n%s", formatLowercaseIssues(combinedIssues))
	}
}

func collectDirIssues(t *testing.T, root string, tagMap map[string]string, trimPrefix string, allowed map[string]struct{}) map[string]map[string]int {
	t.Helper()
	issues := make(map[string]map[string]int)

	err := fs.WalkDir(markdownFiles, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		fileIssues := collectFileIssues(t, p, tagMap, allowed)
		if len(fileIssues) > 0 {
			trimmed := strings.TrimPrefix(p, trimPrefix)
			issues[trimmed] = fileIssues
		}
		return nil
	})

	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return issues
}

func collectLowercaseIssuesInDir(t *testing.T, root string, trimPrefix string, tagMap map[string]string, allowed map[string]struct{}) map[string][]lowercaseMatch {
	t.Helper()
	issues := make(map[string][]lowercaseMatch)

	err := fs.WalkDir(markdownFiles, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		display := p
		if trimPrefix != "" {
			display = strings.TrimPrefix(p, trimPrefix)
		}
		fileIssues := collectLowercaseIssuesInFile(t, p, display, tagMap, allowed)
		if len(fileIssues) > 0 {
			mergeLowercaseIssues(issues, fileIssues)
		}
		return nil
	})

	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return issues
}

func collectFileIssues(t *testing.T, filePath string, tagMap map[string]string, allowed map[string]struct{}) map[string]int {
	t.Helper()
	content, err := markdownFiles.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}

	results := make(map[string]int)
	matches := tagPattern.FindAllString(string(content), -1)
	for _, raw := range matches {
		token := normalizeToken(raw)
		if token == "" {
			continue
		}
		if isKnownTag(token, tagMap) {
			continue
		}
		if _, ok := allowed[token]; ok {
			continue
		}
		results[token]++
	}

	return results
}

func collectLowercaseIssuesInFile(t *testing.T, filePath string, displayPath string, tagMap map[string]string, allowed map[string]struct{}) map[string][]lowercaseMatch {
	t.Helper()
	content, err := markdownFiles.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}

	return findLowercaseTagMatches(string(content), displayPath, tagMap, allowed)
}

func findLowercaseTagMatches(content string, displayPath string, tagMap map[string]string, allowed map[string]struct{}) map[string][]lowercaseMatch {
	results := make(map[string][]lowercaseMatch)
	lines := strings.Split(content, "\n")
	base := path.Base(displayPath)
	tagList := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	for _, tag := range tagList {
		if allowed != nil {
			if _, ok := allowed[tag]; ok {
				continue
			}
		}
		re := tagLowercaseRegexp(tag)
		for _, line := range lines {
			indices := re.FindAllStringIndex(line, -1)
			if len(indices) == 0 {
				continue
			}
			if strings.Contains(line, tag) {
				continue
			}
			for _, loc := range indices {
				match := line[loc[0]:loc[1]]
				if !containsLowercase(match) {
					continue
				}
				results[tag] = append(results[tag], lowercaseMatch{
					File: displayPath,
					Base: base,
					Line: line,
				})
			}
		}
	}
	return results
}

func canonicalizeUpperToken(token string) string {
	if token == "" {
		return ""
	}
	collapsed := strings.Join(strings.Fields(token), " ")
	return canonicalTagName(collapsed)
}

func mergeLowercaseIssues(dest map[string][]lowercaseMatch, src map[string][]lowercaseMatch) {
	for tag, matches := range src {
		dest[tag] = append(dest[tag], matches...)
	}
}

func tagLowercaseRegexp(tag string) *regexp.Regexp {
	if re, ok := lowercaseRegexCache.Load(tag); ok {
		return re.(*regexp.Regexp)
	}
	pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(tag))
	re := regexp.MustCompile(pattern)
	lowercaseRegexCache.Store(tag, re)
	return re
}

func containsLowercase(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			return true
		}
	}
	return false
}

func isKnownTag(token string, tagMap map[string]string) bool {
	if _, ok := tagMap[token]; ok {
		return true
	}
	return false
}

func formatIssues(issues map[string]map[string]int, showPerFile, showTotals bool) string {
	files := make([]string, 0, len(issues))
	for file := range issues {
		files = append(files, file)
	}
	sort.Strings(files)

	var b strings.Builder
	totalCounts := make(map[string]int)

	for _, file := range files {
		tokenCounts := issues[file]
		if len(tokenCounts) == 0 {
			continue
		}

		if showTotals {
			for token, count := range tokenCounts {
				totalCounts[token] += count
			}
		}

		if showPerFile {
			b.WriteString(fmt.Sprintf("%s has %d ALLCAPS strings without tag files:\n", file, len(tokenCounts)))
			tokens := make([]string, 0, len(tokenCounts))
			for token := range tokenCounts {
				tokens = append(tokens, token)
			}
			sort.Strings(tokens)
			for _, token := range tokens {
				count := tokenCounts[token]
				b.WriteString(fmt.Sprintf("  %s (%d)\n", token, count))
			}
			b.WriteString("\n")
		}
	}

	if showTotals && len(totalCounts) > 0 {
		b.WriteString(fmt.Sprintf("%d ALLCAPS strings lack tag files:\n", len(totalCounts)))
		tokens := make([]string, 0, len(totalCounts))
		for token := range totalCounts {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)
		for _, token := range tokens {
			b.WriteString(fmt.Sprintf("  %s (%d)\n", token, totalCounts[token]))
		}
	}

	return strings.TrimSpace(b.String())
}

func formatLowercaseIssues(issues map[string][]lowercaseMatch) string {
	if len(issues) == 0 {
		return ""
	}
	tags := make([]string, 0, len(issues))
	for tag := range issues {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	var b strings.Builder
	for i, tag := range tags {
		matches := issues[tag]
		if len(matches) == 0 {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(tag)
		b.WriteString("\n")
		for _, match := range matches {
			b.WriteString(fmt.Sprintf("%s: %s\n", match.Base, match.Line))
		}
	}

	return strings.TrimSpace(b.String())
}

func loadImportedSessions(t *testing.T) []string {
	t.Helper()
	const importedSessionsPath = "imported-sessions"

	data, err := os.ReadFile(importedSessionsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", importedSessionsPath, err)
	}

	var sessions []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sessions = append(sessions, line)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", importedSessionsPath, err)
	}

	return sessions
}

func loadAllowedAllCaps(t *testing.T) map[string]struct{} {
	t.Helper()

	data, err := os.ReadFile("tagignore")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		t.Fatalf("read tagignore: %v", err)
	}

	allowed := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = struct{}{}
		upper := strings.ToUpper(line)
		allowed[upper] = struct{}{}
		canonical := canonicalTagName(upper)
		if canonical != "" {
			allowed[canonical] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan tagignore: %v", err)
	}

	return allowed
}

func summaryTagsByCategory(t *testing.T) map[string][]string {
	t.Helper()

	root := path.Join(notesRoot, summaryPhysicalDir)
	top, err := fs.ReadDir(markdownFiles, root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}

	tagCategories := make(map[string][]string)

	for _, entry := range top {
		if !entry.IsDir() {
			continue
		}
		category := entry.Name()
		categoryPath := path.Join(root, category)
		files, err := fs.ReadDir(markdownFiles, categoryPath)
		if err != nil {
			t.Fatalf("read %s: %v", categoryPath, err)
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
				continue
			}
			tag := strings.TrimSuffix(file.Name(), ".md")
			tagCategories[tag] = append(tagCategories[tag], category)
		}
	}

	return tagCategories
}

func collectSessionTags(t *testing.T) map[string]struct{} {
	t.Helper()

	tags := make(map[string]struct{})
	err := fs.WalkDir(markdownFiles, "notes/sessions", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}

		content, err := markdownFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		matches := tagPattern.FindAllString(string(content), -1)
		for _, raw := range matches {
			token := normalizeToken(raw)
			if token == "" {
				continue
			}
			tags[token] = struct{}{}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk sessions: %v", err)
	}

	return tags
}
