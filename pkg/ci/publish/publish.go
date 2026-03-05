package publish

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"monks.co/pkg/depgraph"
)

// TopoSort returns the public directories in dependency order
// (dependencies before dependents).
func TopoSort(publicDirs map[string]bool, graph map[string][]string) ([]string, error) {
	// Build the subgraph of only public packages.
	inDegree := map[string]int{}
	edges := map[string][]string{}
	for dir := range publicDirs {
		inDegree[dir] = 0
	}
	for dir := range publicDirs {
		for _, dep := range graph[dir] {
			if publicDirs[dep] {
				edges[dep] = append(edges[dep], dir)
				inDegree[dir]++
			}
		}
	}

	// Kahn's algorithm.
	var queue []string
	for dir := range publicDirs {
		if inDegree[dir] == 0 {
			queue = append(queue, dir)
		}
	}
	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		dependents := edges[node]
		sort.Strings(dependents)
		for _, dep := range dependents {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(result) != len(publicDirs) {
		return nil, fmt.Errorf("cycle detected in public dependency graph")
	}
	return result, nil
}

// GitEnv returns environment variables for running git commands
// with jj's git backend.
func GitEnv(root string) []string {
	gitDir := filepath.Join(root, ".jj", "repo", "store", "git")
	if _, err := os.Stat(gitDir); err != nil {
		return os.Environ()
	}
	env := os.Environ()
	env = append(env, "GIT_DIR="+gitDir, "GIT_WORK_TREE="+root)
	return env
}

// CloneSource returns the path to clone from. For jj repos this is the
// internal git dir; for regular git repos it's the root.
func CloneSource(root string) string {
	gitDir := filepath.Join(root, ".jj", "repo", "store", "git")
	if _, err := os.Stat(gitDir); err == nil {
		return gitDir
	}
	return root
}

// SubtreeSplit runs git subtree split for a directory prefix,
// returning the SHA of the split commit.
func SubtreeSplit(root, dir string) (string, error) {
	cmd := exec.Command("git", "subtree", "split", "--prefix="+dir)
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git subtree split %s: %s", dir, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git subtree split %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// MirrorExists checks if a GitHub repo exists using gh.
func MirrorExists(mirror string) bool {
	cmd := exec.Command("gh", "repo", "view", mirror, "--json", "name")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// CreateMirror creates a public GitHub repo.
func CreateMirror(w io.Writer, mirror string) error {
	parts := strings.SplitN(mirror, "/", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid mirror: %s", mirror)
	}
	repoSlug := parts[1] + "/" + parts[2]
	cmd := exec.Command("gh", "repo", "create", repoSlug, "--public", "--confirm")
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// PushToMirror pushes a split SHA to a mirror repo's main branch.
func PushToMirror(w io.Writer, root, sha, mirror string) error {
	url := "https://" + mirror + ".git"
	cmd := exec.Command("git", "push", url, sha+":refs/heads/main", "--force")
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// PushTagToMirror pushes a tag to a mirror repo.
func PushTagToMirror(w io.Writer, root, sha, tag, mirror string) error {
	url := "https://" + mirror + ".git"
	cmd := exec.Command("git", "push", url, sha+":refs/tags/"+tag, "--force")
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// FindMonorepoTags returns tags for a directory prefix.
// E.g., for dir "pkg/serve", finds tags like "pkg/serve/v1.0.0".
func FindMonorepoTags(root, dir string) ([]string, error) {
	prefix := dir + "/v"
	cmd := exec.Command("git", "tag", "-l", prefix+"*")
	cmd.Dir = root
	cmd.Env = GitEnv(root)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var tags []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags, nil
}

// MirrorTag converts a monorepo tag to a mirror tag.
// "pkg/serve/v1.0.0" -> "v1.0.0"
func MirrorTag(monorepoTag, dir string) string {
	return strings.TrimPrefix(monorepoTag, dir+"/")
}

// FilterRepo clones the repo to a temp dir, runs git-filter-repo to
// keep only the specified paths, and pushes the result to the mirror.
func FilterRepo(w io.Writer, root string, dirs []string, mirror string) error {
	if _, err := exec.LookPath("git-filter-repo"); err != nil {
		return fmt.Errorf("git-filter-repo not found; install with: pip install git-filter-repo")
	}

	tmpDir, err := os.MkdirTemp("", "publish-filter-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	src := CloneSource(root)
	cloneCmd := exec.Command("git", "clone", "--no-local", src, tmpDir)
	cloneCmd.Stdout = w
	cloneCmd.Stderr = w
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("cloning repo: %w", err)
	}

	args := []string{"filter-repo"}
	for _, dir := range dirs {
		args = append(args, "--path", dir)
	}
	args = append(args, "--force")

	filterCmd := exec.Command("git", args...)
	filterCmd.Dir = tmpDir
	filterCmd.Stdout = w
	filterCmd.Stderr = w
	if err := filterCmd.Run(); err != nil {
		return fmt.Errorf("git filter-repo: %w", err)
	}

	url := "https://" + mirror + ".git"
	pushCmd := exec.Command("git", "push", url, "HEAD:refs/heads/main", "--force")
	pushCmd.Dir = tmpDir
	pushCmd.Stdout = w
	pushCmd.Stderr = w
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("pushing to %s: %w", mirror, err)
	}

	for _, dir := range dirs {
		tagsCmd := exec.Command("git", "tag", "-l", dir+"/v*")
		tagsCmd.Dir = tmpDir
		out, err := tagsCmd.Output()
		if err != nil {
			continue
		}
		for tag := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if tag == "" {
				continue
			}
			fmt.Fprintf(w, "  pushing tag %s\n", tag)
			tagPush := exec.Command("git", "push", url, "refs/tags/"+tag+":refs/tags/"+tag, "--force")
			tagPush.Dir = tmpDir
			tagPush.Stdout = w
			tagPush.Stderr = w
			if err := tagPush.Run(); err != nil {
				return fmt.Errorf("pushing tag %s: %w", tag, err)
			}
		}
	}

	return nil
}

// Analyze separates packages into explicit-mirror and default-mirror groups.
func Analyze(root string, cfg *Config) (explicitPkgs []Package, defaultMirrorDirs []string, err error) {
	graph, err := depgraph.BuildDepGraph(root)
	if err != nil {
		return nil, nil, fmt.Errorf("building dep graph: %w", err)
	}

	publicDirs := cfg.PublicDirs()
	order, err := TopoSort(publicDirs, graph)
	if err != nil {
		return nil, nil, err
	}

	for _, dir := range order {
		pkg := cfg.PackageByDir(dir)
		if pkg != nil && pkg.Mirror != "" {
			explicitPkgs = append(explicitPkgs, *pkg)
		} else {
			defaultMirrorDirs = append(defaultMirrorDirs, dir)
		}
	}
	return explicitPkgs, defaultMirrorDirs, nil
}

// PublishExplicitMirror publishes a single package with an explicit mirror
// via git subtree split.
func PublishExplicitMirror(w io.Writer, root string, pkg Package) error {
	fmt.Fprintf(w, "publishing %s -> %s (subtree split)\n", pkg.Dir, pkg.Mirror)

	if !MirrorExists(pkg.Mirror) {
		fmt.Fprintf(w, "  creating mirror repo %s\n", pkg.Mirror)
		if err := CreateMirror(w, pkg.Mirror); err != nil {
			return fmt.Errorf("creating mirror %s: %w", pkg.Mirror, err)
		}
	}

	fmt.Fprintf(w, "  splitting %s...\n", pkg.Dir)
	sha, err := SubtreeSplit(root, pkg.Dir)
	if err != nil {
		return fmt.Errorf("splitting %s: %w", pkg.Dir, err)
	}
	fmt.Fprintf(w, "  split SHA: %s\n", sha)

	fmt.Fprintf(w, "  pushing to %s\n", pkg.Mirror)
	if err := PushToMirror(w, root, sha, pkg.Mirror); err != nil {
		return fmt.Errorf("pushing %s: %w", pkg.Dir, err)
	}

	tags, err := FindMonorepoTags(root, pkg.Dir)
	if err != nil {
		return fmt.Errorf("finding tags for %s: %w", pkg.Dir, err)
	}
	for _, tag := range tags {
		mTag := MirrorTag(tag, pkg.Dir)
		fmt.Fprintf(w, "  pushing tag %s -> %s\n", tag, mTag)
		if err := PushTagToMirror(w, root, sha, mTag, pkg.Mirror); err != nil {
			return fmt.Errorf("pushing tag %s: %w", tag, err)
		}
	}

	return nil
}

// PublishDefaultMirror publishes packages without explicit mirrors via
// git-filter-repo to the default mirror repo.
func PublishDefaultMirror(w io.Writer, root string, dirs []string, mirror string) error {
	fmt.Fprintf(w, "publishing %d packages -> %s (filter-repo)\n", len(dirs), mirror)
	for _, dir := range dirs {
		fmt.Fprintf(w, "  %s\n", dir)
	}

	if !MirrorExists(mirror) {
		fmt.Fprintf(w, "  creating mirror repo %s\n", mirror)
		if err := CreateMirror(w, mirror); err != nil {
			return fmt.Errorf("creating mirror %s: %w", mirror, err)
		}
	}

	if err := FilterRepo(w, root, dirs, mirror); err != nil {
		return fmt.Errorf("filter-repo: %w", err)
	}

	return nil
}

// Run executes the full publish flow for all public packages.
func Run(w io.Writer, root string, cfg *Config, dryRun bool) error {
	explicitPkgs, defaultMirrorDirs, err := Analyze(root, cfg)
	if err != nil {
		return err
	}

	for _, pkg := range explicitPkgs {
		if dryRun {
			fmt.Fprintf(w, "publishing %s -> %s (subtree split)\n", pkg.Dir, pkg.Mirror)
			fmt.Fprintf(w, "  [dry-run] would split, create repo if needed, push\n")
			continue
		}
		if err := PublishExplicitMirror(w, root, pkg); err != nil {
			return err
		}
	}

	if len(defaultMirrorDirs) > 0 && cfg.DefaultMirror != "" {
		if dryRun {
			fmt.Fprintf(w, "publishing %d packages -> %s (filter-repo)\n", len(defaultMirrorDirs), cfg.DefaultMirror)
			fmt.Fprintf(w, "  [dry-run] would clone, filter, push\n")
		} else {
			if err := PublishDefaultMirror(w, root, defaultMirrorDirs, cfg.DefaultMirror); err != nil {
				return err
			}
		}
	}

	return nil
}
