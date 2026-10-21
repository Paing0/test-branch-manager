package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RepoInfo represents a discovered Git repository.
type RepoInfo struct {
	Name string
	Path string
}

// Repository represents a Git repository and provides operations on it.
type Repository struct {
	Path     string
	Name     string
	Executor *Executor
}

// NewRepository creates a Repository for the given path.
func NewRepository(path string, executor *Executor) *Repository {
	return &Repository{
		Path:     path,
		Name:     filepath.Base(path),
		Executor: executor,
	}
}

// DiscoverRepos scans the given directory for Git repositories.
// It checks the directory itself and its immediate children.
func DiscoverRepos(dir string) ([]RepoInfo, error) {
	var repos []RepoInfo

	// Check the directory itself
	if isGitRepo(dir) {
		repos = append(repos, RepoInfo{
			Name: filepath.Base(dir),
			Path: dir,
		})
	}

	// Check immediate child directories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return repos, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childPath := filepath.Join(dir, entry.Name())
		if isGitRepo(childPath) {
			repos = append(repos, RepoInfo{
				Name: entry.Name(),
				Path: childPath,
			})
		}
	}

	return repos, nil
}

// isGitRepo checks if a directory is a valid Git repository.
// It handles both regular .git directories and .git files (worktrees/submodules).
func isGitRepo(dir string) bool {
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	// .git can be a directory (regular repo) or a file (worktree/submodule)
	return info.IsDir() || info.Mode().IsRegular()
}

// Validate checks that the repository is in a safe state for the workflow.
func (r *Repository) Validate(ctx context.Context) error {
	// Check for uncommitted changes
	hasChanges, err := r.HasUncommittedChanges(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("repository has uncommitted changes; please commit or stash them first")
	}

	// Check for merge in progress
	merging, err := r.IsMergeInProgress(ctx)
	if err != nil {
		return fmt.Errorf("failed to check merge state: %w", err)
	}
	if merging {
		return fmt.Errorf("a merge is already in progress; please complete or abort it first")
	}

	// Check for rebase in progress
	rebasing, err := r.IsRebaseInProgress(ctx)
	if err != nil {
		return fmt.Errorf("failed to check rebase state: %w", err)
	}
	if rebasing {
		return fmt.Errorf("a rebase is in progress; please complete or abort it first")
	}

	// Check for detached HEAD
	detached, err := r.IsDetachedHead(ctx)
	if err != nil {
		return fmt.Errorf("failed to check HEAD state: %w", err)
	}
	if detached {
		return fmt.Errorf("HEAD is detached; please check out a branch first")
	}

	return nil
}

// HasUncommittedChanges checks for staged or unstaged changes in tracked files.
func (r *Repository) HasUncommittedChanges(ctx context.Context) (bool, error) {
	result, err := r.Executor.Run(ctx, r.Path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return result.Stdout != "", nil
}

// IsMergeInProgress checks if a merge is currently in progress.
func (r *Repository) IsMergeInProgress(ctx context.Context) (bool, error) {
	// Check for MERGE_HEAD file
	mergeHead := filepath.Join(r.Path, ".git", "MERGE_HEAD")
	if _, err := os.Stat(mergeHead); err == nil {
		return true, nil
	}

	// Also handle .git file (worktree) by using rev-parse
	result, _ := r.Executor.RunAllowFailure(ctx, r.Path, "rev-parse", "--git-dir")
	if result != nil && result.Stdout != "" {
		gitDir := result.Stdout
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(r.Path, gitDir)
		}
		mh := filepath.Join(gitDir, "MERGE_HEAD")
		if _, err := os.Stat(mh); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// IsRebaseInProgress checks if a rebase is currently in progress.
func (r *Repository) IsRebaseInProgress(ctx context.Context) (bool, error) {
	result, _ := r.Executor.RunAllowFailure(ctx, r.Path, "rev-parse", "--git-dir")
	if result == nil || result.Stdout == "" {
		return false, nil
	}
	gitDir := result.Stdout
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(r.Path, gitDir)
	}

	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, dir)); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// IsDetachedHead checks if HEAD is detached.
func (r *Repository) IsDetachedHead(ctx context.Context) (bool, error) {
	result, err := r.Executor.Run(ctx, r.Path, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		// Exit code 1 means detached HEAD
		if result != nil && result.ExitCode == 1 {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func (r *Repository) CurrentBranch(ctx context.Context) (string, error) {
	result, err := r.Executor.Run(ctx, r.Path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	if result.Stdout == "HEAD" {
		return "", fmt.Errorf("HEAD is detached")
	}
	return result.Stdout, nil
}

// RemoteExists checks if a remote with the given name is configured.
func (r *Repository) RemoteExists(ctx context.Context, remote string) (bool, error) {
	result, err := r.Executor.Run(ctx, r.Path, "remote")
	if err != nil {
		return false, fmt.Errorf("failed to list remotes: %w", err)
	}
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == remote {
			return true, nil
		}
	}
	return false, nil
}

// ListRemotes returns the list of configured remote names.
func (r *Repository) ListRemotes(ctx context.Context) ([]string, error) {
	result, err := r.Executor.Run(ctx, r.Path, "remote")
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}
	if result.Stdout == "" {
		return nil, nil
	}
	var remotes []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

// GetGitDir returns the path to the actual .git directory.
func (r *Repository) GetGitDir(ctx context.Context) (string, error) {
	result, err := r.Executor.Run(ctx, r.Path, "rev-parse", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git dir: %w", err)
	}
	gitDir := result.Stdout
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(r.Path, gitDir)
	}
	return gitDir, nil
}
