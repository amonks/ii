package workspace

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"monks.co/incrementum/internal/config"
	"monks.co/incrementum/internal/db"
	"monks.co/incrementum/internal/jj"
	"monks.co/incrementum/internal/paths"
	internalstrings "monks.co/incrementum/internal/strings"
)

// Pool manages a pool of jujutsu workspaces.
//
// A Pool maintains workspaces in a shared location and tracks which workspaces
// are currently acquired. Multiple processes can safely use the same Pool
// concurrently through SQLite coordination.
type Pool struct {
	store         *Store
	workspacesDir string
	jj            *jj.Client
	close         func() error
}

// Options configures a workspace pool.
type Options struct {
	// StateDir is the directory where the SQLite database is stored.
	// Defaults to ~/.local/state/incrementum if empty.
	StateDir string

	// WorkspacesDir is the directory where workspaces are created.
	// Defaults to ~/.local/share/incrementum/workspaces if empty.
	WorkspacesDir string
}

// Open creates a new Pool with default options.
// State is stored in ~/.local/state/incrementum and workspaces in
// ~/.local/share/incrementum/workspaces.
func Open() (*Pool, error) {
	return OpenWithOptions(Options{})
}

// OpenWithOptions creates a new Pool with custom options.
func OpenWithOptions(opts Options) (*Pool, error) {
	stateDir, err := paths.ResolveWithDefault(opts.StateDir, paths.DefaultStateDir)
	if err != nil {
		return nil, err
	}

	workspacesDir, err := paths.ResolveWithDefault(opts.WorkspacesDir, paths.DefaultWorkspacesDir)
	if err != nil {
		return nil, err
	}

	dbPath, err := resolveDBPath(stateDir)
	if err != nil {
		return nil, err
	}

	dbStore, err := db.Open(dbPath, db.OpenOptions{LegacyJSONPath: filepath.Join(stateDir, "state.json")})
	if err != nil {
		return nil, err
	}

	pool := NewPool(dbStore.SqlDB(), workspacesDir)
	pool.SetCloseFunc(dbStore.Close)
	return pool, nil
}

// NewPool constructs a pool using an existing database connection.
func NewPool(sqlDB *sql.DB, workspacesDir string) *Pool {
	return &Pool{
		store:         NewStore(sqlDB),
		workspacesDir: workspacesDir,
		jj:            jj.New(),
	}
}

// SetCloseFunc configures the close callback for a pool.
func (p *Pool) SetCloseFunc(closeFn func() error) {
	if p == nil {
		return
	}
	p.close = closeFn
}

// RepoSlug returns the repo slug used for state storage.
func (p *Pool) RepoSlug(repoPath string) (string, error) {
	repoName, err := p.store.GetOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}
	return repoName, nil
}

// AcquireOptions configures a workspace acquire operation.
type AcquireOptions struct {
	// Rev is the jj revision to base a new change on. Defaults to "@" if empty.
	Rev string

	// Purpose describes why the workspace is being acquired.
	// It must be a single-line string.
	Purpose string

	// NewChangeMessage is an optional description to apply when a new change
	// is created because the requested revision is immutable.
	NewChangeMessage string

	// SkipHooks suppresses on-create hook execution and provisioning marking.
	// Use this when the workspace content will differ from the main tree
	// (e.g. the todo store edits to an orphan bookmark immediately after acquire).
	SkipHooks bool
}

// ValidateAcquirePurpose ensures the purpose is present and single-line.
func ValidateAcquirePurpose(purpose string) error {
	if internalstrings.IsBlank(purpose) {
		return fmt.Errorf("purpose is required")
	}
	if strings.ContainsAny(purpose, "\r\n") {
		return fmt.Errorf("purpose must be a single line")
	}
	return nil
}

// Acquire obtains a workspace from the pool for the given repository.
//
// If an available workspace exists, it will be reused. Otherwise, a new
// workspace is created. The workspace is checked out to a new change based on
// the specified revision (or @ by default).
//
// The returned path is the root directory of the acquired workspace.
// Call Release when done to return the workspace to the pool.
//
// If the repository contains an incrementum.toml or .incrementum/config.toml
// configuration file, the on-create hooks run on every acquire.

func (p *Pool) Acquire(repoPath string, opts AcquireOptions) (string, error) {
	// Apply defaults
	if opts.Rev == "" {
		opts.Rev = "@"
	}
	if err := ValidateAcquirePurpose(opts.Purpose); err != nil {
		return "", err
	}

	repoName, err := p.store.GetOrCreateRepoName(repoPath)
	if err != nil {
		return "", fmt.Errorf("get repo name: %w", err)
	}

	var wsPath string
	var wsName string
	var needsCreate bool
	var needsProvision bool

	now := time.Now()
	available, err := p.store.AcquireAvailableWorkspace(repoName, WorkspaceInfo{
		Repo:          repoName,
		Purpose:       opts.Purpose,
		Rev:           opts.Rev,
		Status:        StatusAcquired,
		AcquiredByPID: os.Getpid(),
		AcquiredAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		return "", err
	}
	if available != nil {
		wsPath = paths.NormalizePath(available.Path)
		wsName = available.Name
		needsProvision = !available.Provisioned
	} else {
		wsName, err = p.store.NextWorkspaceName(repoName)
		if err != nil {
			return "", err
		}
		wsPath = paths.NormalizePath(filepath.Join(p.workspacesDir, repoName, wsName))
		needsCreate = true
		needsProvision = true
		if err := p.store.InsertWorkspace(WorkspaceInfo{
			Name:          wsName,
			Repo:          repoName,
			Path:          wsPath,
			Purpose:       opts.Purpose,
			Rev:           opts.Rev,
			Status:        StatusAcquired,
			AcquiredByPID: os.Getpid(),
			AcquiredAt:    now,
			CreatedAt:     now,
			UpdatedAt:     now,
			Provisioned:   false,
		}); err != nil {
			return "", err
		}
	}

	// Create the workspace directory if needed
	if needsCreate {
		if err := os.MkdirAll(filepath.Dir(wsPath), 0755); err != nil {
			return "", fmt.Errorf("create workspace parent dir: %w", err)
		}

		if err := p.jj.WorkspaceAdd(repoPath, wsName, wsPath); err != nil {
			// Clean up state on failure
			if deleteErr := p.store.DeleteWorkspace(repoName, wsName); deleteErr != nil {
				return "", fmt.Errorf("jj workspace add: %w; cleanup failed: %v", err, deleteErr)
			}
			return "", fmt.Errorf("jj workspace add: %w", err)
		}
	}

	// Resolve the revision in the source repo first. This is necessary because
	// symbolic refs like "@" have different meanings in the workspace vs source.
	resolvedRev, err := p.jj.ChangeIDAt(repoPath, opts.Rev)
	if err != nil {
		resolvedRev = opts.Rev // Fall back to original if resolution fails
	}

	newChange := func(parentRev string) (string, error) {
		if !internalstrings.IsBlank(opts.NewChangeMessage) {
			return p.jj.NewChangeWithMessage(wsPath, parentRev, opts.NewChangeMessage)
		}
		return p.jj.NewChange(wsPath, parentRev)
	}

	actualRev, err := newChange(resolvedRev)
	if err != nil {
		if isMissingRevisionError(err) && looksLikeChangeID(opts.Rev) {
			// Fall back to @ resolved in the source repo
			fallbackRev, resolveErr := p.jj.ChangeIDAt(repoPath, "@")
			if resolveErr != nil {
				fallbackRev = "@"
			}
			actualRev, err = newChange(fallbackRev)
		}
		if err != nil {
			return "", fmt.Errorf("jj new: %w", err)
		}
	}

	if actualRev != opts.Rev {
		if err := p.store.UpdateWorkspaceRevision(repoName, wsName, actualRev, time.Now()); err != nil {
			p.Release(wsPath)
			return "", fmt.Errorf("update workspace rev: %w", err)
		}
	}

	// Load config and run hooks (skipped for non-main-tree acquires like todo store)
	if !opts.SkipHooks {
		cfg, err := config.Load(repoPath)
		if err != nil {
			return "", fmt.Errorf("load config: %w", err)
		}

		// Run on-create script for every acquire
		if err := config.RunScript(wsPath, cfg.Workspace.OnCreate); err != nil {
			p.Release(wsPath)
			return "", fmt.Errorf("on-create script: %w", err)
		}

		// Mark as provisioned if needed
		if needsProvision {
			if err := p.store.MarkWorkspaceProvisioned(repoName, wsName); err != nil {
				return "", fmt.Errorf("mark workspace provisioned: %w", err)
			}
		}
	}

	return wsPath, nil
}

// Release returns a workspace to the pool, making it available for reuse.
//
// After releasing, the workspace path should no longer be used. The workspace
// directory remains on disk and may be acquired again later.
func (p *Pool) Release(wsPath string) error {
	return p.releaseToAvailable(wsPath)
}

func (p *Pool) releaseToAvailable(wsPath string) error {
	if _, err := p.jj.NewChange(wsPath, "root()"); err != nil {
		return fmt.Errorf("jj new root(): %w", err)
	}

	wsPath = paths.NormalizePath(wsPath)
	if err := p.store.ReleaseWorkspace(wsPath, time.Now()); err != nil {
		return fmt.Errorf("release workspace: %w", err)
	}
	return nil
}

// ReleaseByName returns a workspace to the pool by name.
func (p *Pool) ReleaseByName(repoPath, wsName string) error {
	repoName, err := p.store.GetOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	ws, err := p.store.GetWorkspaceByName(repoName, wsName)
	if err != nil {
		return fmt.Errorf("get workspace: %w", err)
	}

	return p.releaseToAvailable(ws.Path)
}

// Info contains information about a workspace.
type Info struct {
	// Name is the workspace identifier (e.g., "ws-001").
	Name string

	// Path is the absolute path to the workspace directory.
	Path string

	// Purpose describes why the workspace was acquired.
	Purpose string

	// Rev is the jj revision the workspace was opened to.
	Rev string

	// Status indicates whether the workspace is available or acquired.
	Status Status

	// AcquiredByPID is the process ID that acquired this workspace.
	// Zero if not acquired.
	AcquiredByPID int

	// AcquiredAt is when the workspace was acquired.
	// Zero if not acquired.
	AcquiredAt time.Time

	// CreatedAt is when the workspace acquisition started.
	CreatedAt time.Time

	// UpdatedAt is when the workspace was last released.
	UpdatedAt time.Time
}

// List returns information about all workspaces for the given repository.
//
// The returned slice includes both available and acquired workspaces.

func (p *Pool) List(repoPath string) ([]Info, error) {
	repoName, err := p.store.GetOrCreateRepoName(repoPath)
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	workspaces, err := p.store.ListWorkspaces(repoName)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	var items []Info

	for _, ws := range workspaces {
		item := Info{
			Name:          ws.Name,
			Path:          ws.Path,
			Purpose:       ws.Purpose,
			Rev:           ws.Rev,
			Status:        ws.Status,
			AcquiredByPID: ws.AcquiredByPID,
			AcquiredAt:    ws.AcquiredAt,
			CreatedAt:     ws.CreatedAt,
			UpdatedAt:     ws.UpdatedAt,
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return workspaceStatusRank(items[i].Status) < workspaceStatusRank(items[j].Status)
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Path < items[j].Path
	})

	return items, nil
}

func workspaceStatusRank(status Status) int {
	switch status {
	case StatusAcquired:
		return 0
	case StatusAvailable:
		return 1
	default:
		return 2
	}
}

func isMissingRevisionError(err error) bool {
	if err == nil {
		return false
	}
	return internalstrings.ContainsAnyLower(err.Error(), "doesn't exist", "does not exist")
}

func looksLikeChangeID(rev string) bool {
	if len(rev) < 12 {
		return false
	}
	for _, r := range rev {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'z':
		default:
			return false
		}
	}
	return true
}

// RepoRoot returns the jj repository root for the given path.
//
// This can be used to find the repository root before calling Acquire.
// Returns an error if the path is not inside a jj repository.
// The returned path is normalized to handle macOS symlinks like /private/var.
func RepoRoot(path string) (string, error) {
	client := jj.New()
	root, err := client.WorkspaceRoot(path)
	if err != nil {
		return "", err
	}
	return paths.NormalizePath(root), nil
}

func resolveDBPath(stateDir string) (string, error) {
	if stateDir != "" {
		return filepath.Join(stateDir, "state.db"), nil
	}
	return paths.DefaultDBPath()
}

// RepoRootFromPath returns the source repo root for a workspace or repo path.
// If the path is a workspace, it resolves to the original repo using state.
func RepoRootFromPath(path string) (string, error) {
	return repoRootFromPathWithOptions(path, Options{})
}

// RepoRootFromPathWithOptions is like RepoRootFromPath with custom options.
func RepoRootFromPathWithOptions(path string, opts Options) (string, error) {
	return repoRootFromPathWithOptions(path, opts)
}

// WorkspaceNameForPath returns the workspace name for a workspace path.
// Returns ErrWorkspaceRootNotFound if the path is not a workspace.
func (p *Pool) WorkspaceNameForPath(path string) (string, error) {
	root, err := RepoRoot(path)
	if err != nil {
		return "", ErrWorkspaceRootNotFound
	}

	root = filepath.Clean(root)
	ws, err := p.store.GetWorkspaceByPath(root)
	if err != nil {
		return "", err
	}
	if ws == nil {
		return "", ErrRepoPathNotFound
	}
	return ws.Name, nil
}

func repoRootFromPathWithOptions(path string, opts Options) (string, error) {
	root, err := RepoRoot(path)
	if err != nil {
		return "", ErrWorkspaceRootNotFound
	}

	pool, err := OpenWithOptions(opts)
	if err != nil {
		return "", fmt.Errorf("open workspace pool: %w", err)
	}
	defer pool.Close()

	repoPath, found, err := pool.store.RepoPathForWorkspace(root)
	if err != nil {
		return "", err
	}

	if found {
		if repoPath == "" {
			return "", ErrRepoPathNotFound
		}
		return repoPath, nil
	}

	rel, err := filepath.Rel(pool.workspacesDir, root)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return "", ErrRepoPathNotFound
	}

	return root, nil
}

// DestroyAll removes all workspaces for the given repository.
//
// This deletes both the state entries and the workspace directories on disk.
// It also runs "jj workspace forget" to unregister each workspace from the
// source repository.
func (p *Pool) DestroyAll(repoPath string) error {
	repoName, err := p.store.GetOrCreateRepoName(repoPath)
	if err != nil {
		return fmt.Errorf("get repo name: %w", err)
	}

	workspaces, repoSourcePath, err := p.store.DeleteWorkspaces(repoName)
	if err != nil {
		return err
	}

	// Forget workspaces from jj and delete directories
	var errs []error
	for _, ws := range workspaces {
		// Try to forget from jj (may fail if source repo is gone)
		if repoSourcePath != "" {
			if err := p.jj.WorkspaceForget(repoSourcePath, ws.Name); err != nil {
				// Non-fatal - the workspace might already be forgotten or the repo gone
				errs = append(errs, fmt.Errorf("forget workspace %s: %w", ws.Name, err))
			}
		}

		// Delete the workspace directory
		if err := os.RemoveAll(ws.Path); err != nil {
			errs = append(errs, fmt.Errorf("remove workspace %s: %w", ws.Path, err))
		}
	}

	// Also try to remove the repo's workspace directory if empty
	repoWorkspacesDir := filepath.Join(p.workspacesDir, repoName)
	os.Remove(repoWorkspacesDir) // Ignore error - may not be empty or exist

	if len(errs) > 0 {
		// Return first error but log intent that some cleanup failed
		return errs[0]
	}

	return nil
}
