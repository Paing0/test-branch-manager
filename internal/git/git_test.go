package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"test-branch-creator/internal/history"
)

// setupTestRepo creates a temporary Git repository for testing.
// Returns the repo path and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize a git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")

	// Create initial commit
	writeFile(t, filepath.Join(dir, "README.md"), "# Test Repo\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")

	return dir, cleanup
}

// setupTestRepoWithRemote creates a test repo with a bare remote.
// Returns repo path, remote path, and cleanup function.
func setupTestRepoWithRemote(t *testing.T) (string, string, func()) {
	t.Helper()

	// Create bare remote
	remoteDir, err := os.MkdirTemp("", "git-remote-*")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, remoteDir, "init", "--bare")

	// Create working repo
	repoDir, err := os.MkdirTemp("", "git-work-*")
	if err != nil {
		os.RemoveAll(remoteDir)
		t.Fatal(err)
	}

	cleanup := func() {
		os.RemoveAll(repoDir)
		os.RemoveAll(remoteDir)
	}

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "test@test.com")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "remote", "add", "origin", remoteDir)

	// Create initial commit on main and push
	writeFile(t, filepath.Join(repoDir, "README.md"), "# Test Repo\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "initial commit")
	runGit(t, repoDir, "push", "origin", "HEAD")

	// Create develop branch and push
	runGit(t, repoDir, "switch", "-c", "develop")
	writeFile(t, filepath.Join(repoDir, "develop.txt"), "develop branch\n")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "develop commit")
	runGit(t, repoDir, "push", "origin", "develop")

	return repoDir, remoteDir, cleanup
}

// runGit executes a git command in the specified directory.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// writeFile creates or overwrites a file with the given content.
func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	return NewExecutor(history.NewRecorder(), false)
}

// --- Unit Tests ---

func TestDiscoverRepos(t *testing.T) {
	// Create a parent dir with some child repos
	parent, err := os.MkdirTemp("", "discover-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)

	// Create two child repos
	repo1 := filepath.Join(parent, "repo-one")
	repo2 := filepath.Join(parent, "repo-two")
	nonRepo := filepath.Join(parent, "not-a-repo")

	os.MkdirAll(repo1, 0o755)
	os.MkdirAll(repo2, 0o755)
	os.MkdirAll(nonRepo, 0o755)

	runGitDir(t, repo1, "init")
	runGitDir(t, repo2, "init")

	repos, err := DiscoverRepos(parent)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %+v", len(repos), repos)
	}

	// Check names
	names := map[string]bool{}
	for _, r := range repos {
		names[r.Name] = true
	}
	if !names["repo-one"] || !names["repo-two"] {
		t.Errorf("expected repo-one and repo-two, got %+v", names)
	}
}

func TestDiscoverRepos_CurrentDir(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	repos, err := DiscoverRepos(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (current dir), got %d", len(repos))
	}
	if repos[0].Path != repoDir {
		t.Errorf("expected path %q, got %q", repoDir, repos[0].Path)
	}
}

func TestDiscoverRepos_NoRepos(t *testing.T) {
	dir, err := os.MkdirTemp("", "no-repos-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	repos, err := DiscoverRepos(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestDiscoverRepos_GitFile(t *testing.T) {
	// Simulate a repo with a .git file (like a worktree)
	dir, err := os.MkdirTemp("", "git-file-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	child := filepath.Join(dir, "worktree-repo")
	os.MkdirAll(child, 0o755)
	// Write a .git file (not directory)
	os.WriteFile(filepath.Join(child, ".git"), []byte("gitdir: /somewhere/else\n"), 0o644)

	repos, err := DiscoverRepos(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(repos) != 1 {
		t.Fatalf("expected 1 repo (git file), got %d", len(repos))
	}
}

func runGitDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func TestIsGitRepo(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		expected bool
	}{
		{
			name: "real git repo",
			setup: func(t *testing.T) string {
				dir, _ := setupTestRepo(t)
				return dir
			},
			expected: true,
		},
		{
			name: "non-git directory",
			setup: func(t *testing.T) string {
				dir, _ := os.MkdirTemp("", "nongit-*")
				return dir
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			defer os.RemoveAll(dir)
			got := isGitRepo(dir)
			if got != tt.expected {
				t.Errorf("expected %v, got %v for %s", tt.expected, got, dir)
			}
		})
	}
}

func TestRepository_HasUncommittedChanges(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Clean repo should have no changes
	has, err := repo.HasUncommittedChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected no uncommitted changes in clean repo")
	}

	// Make a change
	writeFile(t, filepath.Join(repoDir, "new.txt"), "something")
	has, err = repo.HasUncommittedChanges(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected uncommitted changes after modifying file")
	}
}

func TestRepository_CurrentBranch(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Default branch could be "main" or "master"
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master, got %q", branch)
	}
}

func TestRepository_RemoteOperations(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Check remote exists
	exists, err := repo.RemoteExists(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected origin remote to exist")
	}

	// Check non-existent remote
	exists, err = repo.RemoteExists(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("expected nonexistent remote to not exist")
	}

	// List remotes
	remotes, err := repo.ListRemotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 1 || remotes[0] != "origin" {
		t.Errorf("expected [origin], got %v", remotes)
	}
}

func TestRepository_BranchOperations(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Fetch
	err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}

	// List remote branches
	branches, err := repo.ListRemoteBranches(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) == 0 {
		t.Fatal("expected at least one remote branch")
	}

	// Check develop exists
	hasDevelop := false
	for _, b := range branches {
		if strings.Contains(b, "develop") {
			hasDevelop = true
			break
		}
	}
	if !hasDevelop {
		t.Errorf("expected develop branch in remote branches: %v", branches)
	}

	// Create branch from remote develop
	err = repo.CreateBranchFrom(ctx, "test/offshore-test-20261021", "origin/develop")
	if err != nil {
		t.Fatal(err)
	}

	// Verify we're on the new branch
	current, err := repo.CurrentBranch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current != "test/offshore-test-20261021" {
		t.Errorf("expected branch test/offshore-test-20261021, got %q", current)
	}

	// Check local branch exists
	exists, err := repo.LocalBranchExists(ctx, "test/offshore-test-20261021")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected local branch to exist")
	}
}

func TestRepository_Validate(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Clean repo should validate
	err := repo.Validate(ctx)
	if err != nil {
		t.Errorf("expected clean repo to validate, got: %v", err)
	}

	// Add uncommitted changes
	writeFile(t, filepath.Join(repoDir, "dirty.txt"), "dirty")
	err = repo.Validate(ctx)
	if err == nil {
		t.Error("expected validation to fail with uncommitted changes")
	}
}

func TestRepository_Merge(t *testing.T) {
	repoDir, remoteDir, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Create a feature branch with some changes
	runGit(t, repoDir, "switch", "-c", "feature/test")
	writeFile(t, filepath.Join(repoDir, "feature.txt"), "feature content")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "feature commit")
	runGit(t, repoDir, "push", "origin", "feature/test")
	_ = remoteDir

	// Go back to develop and create test branch
	runGit(t, repoDir, "switch", "develop")
	err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}

	err = repo.CreateBranchFrom(ctx, "test/offshore-test-20261021", "origin/develop")
	if err != nil {
		t.Fatal(err)
	}

	// Merge the feature branch (should succeed without conflicts)
	result, err := repo.Merge(ctx, "origin/feature/test")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Errorf("expected merge to succeed, got conflicts: %v", result.ConflictedFiles)
	}
}

func TestRepository_MergeConflict(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Create conflicting changes on a feature branch
	runGit(t, repoDir, "switch", "-c", "feature/conflict")
	writeFile(t, filepath.Join(repoDir, "conflict.txt"), "feature version")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "feature conflict")
	runGit(t, repoDir, "push", "origin", "feature/conflict")

	// Create conflicting changes on develop
	runGit(t, repoDir, "switch", "develop")
	writeFile(t, filepath.Join(repoDir, "conflict.txt"), "develop version")
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "develop conflict")
	runGit(t, repoDir, "push", "origin", "develop")

	// Fetch and create test branch from develop
	err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}

	err = repo.CreateBranchFrom(ctx, "test/offshore-test-20261021", "origin/develop")
	if err != nil {
		t.Fatal(err)
	}

	// Merge feature branch — should have conflicts
	result, err := repo.Merge(ctx, "origin/feature/conflict")
	if err != nil {
		t.Fatalf("unexpected error (expected conflict): %v", err)
	}
	if result.Success {
		t.Error("expected merge conflict, but merge succeeded")
	}
	if !result.HasConflicts {
		t.Error("expected HasConflicts to be true")
	}
	if len(result.ConflictedFiles) == 0 {
		t.Error("expected conflicted files")
	}

	// Check conflicts
	hasConflicts, err := repo.HasConflicts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hasConflicts {
		t.Error("expected HasConflicts to be true")
	}

	// Resolve the conflict
	writeFile(t, filepath.Join(repoDir, "conflict.txt"), "resolved version")
	runGit(t, repoDir, "add", "conflict.txt")

	// Now should have no conflicts
	hasConflicts, err = repo.HasConflicts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if hasConflicts {
		t.Error("expected no conflicts after resolution")
	}

	// Needs merge commit
	needs, err := repo.NeedsMergeCommit(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !needs {
		t.Error("expected NeedsMergeCommit to be true")
	}

	// Complete the merge
	err = repo.CommitMerge(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Merge should be complete
	merging, err := repo.IsMergeInProgress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if merging {
		t.Error("expected merge to be complete")
	}
}

func TestRepository_Push(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}

	// Create and push a new branch
	err = repo.CreateBranchFrom(ctx, "test/offshore-test-20261021", "origin/develop")
	if err != nil {
		t.Fatal(err)
	}

	err = repo.Push(ctx, "origin", "test/offshore-test-20261021")
	if err != nil {
		t.Fatal(err)
	}

	// Verify branch exists on remote
	exists, err := repo.RemoteBranchExists(ctx, "origin", "test/offshore-test-20261021")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("expected branch to exist on remote after push")
	}
}

func TestRepository_BranchExistsAnywhere(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// develop exists both locally and on remote
	local, remote, err := repo.BranchExistsAnywhere(ctx, "develop", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if !local {
		t.Error("expected develop to exist locally")
	}
	if !remote {
		t.Error("expected develop to exist on remote")
	}

	// nonexistent branch
	local, remote, err = repo.BranchExistsAnywhere(ctx, "nonexistent/branch", "origin")
	if err != nil {
		t.Fatal(err)
	}
	if local {
		t.Error("expected nonexistent branch to not exist locally")
	}
	if remote {
		t.Error("expected nonexistent branch to not exist on remote")
	}
}

func TestRepository_DetachedHead(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Not detached initially
	detached, err := repo.IsDetachedHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if detached {
		t.Error("expected HEAD to not be detached")
	}

	// Detach HEAD
	runGit(t, repoDir, "checkout", "--detach")
	detached, err = repo.IsDetachedHead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !detached {
		t.Error("expected HEAD to be detached")
	}
}

func TestRepository_ListRemoteBranchesMatching(t *testing.T) {
	repoDir, _, cleanup := setupTestRepoWithRemote(t)
	defer cleanup()

	exec := newTestExecutor(t)
	repo := NewRepository(repoDir, exec)
	ctx := context.Background()

	// Create some test/it-* branches
	runGit(t, repoDir, "switch", "develop")
	runGit(t, repoDir, "switch", "-c", "test/it-20261020")
	runGit(t, repoDir, "push", "origin", "test/it-20261020")
	runGit(t, repoDir, "switch", "-c", "test/it-20261019")
	runGit(t, repoDir, "push", "origin", "test/it-20261019")

	err := repo.Fetch(ctx, "origin")
	if err != nil {
		t.Fatal(err)
	}

	branches, err := repo.ListRemoteBranchesMatching(ctx, "origin", "test/it-*")
	if err != nil {
		t.Fatal(err)
	}

	if len(branches) != 2 {
		t.Errorf("expected 2 matching branches, got %d: %v", len(branches), branches)
	}
}

func TestExecutor_RecordsHistory(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	recorder := history.NewRecorder()
	exec := NewExecutor(recorder, false)
	ctx := context.Background()

	// Run a command
	_, err := exec.Run(ctx, repoDir, "status")
	if err != nil {
		t.Fatal(err)
	}

	// Check history
	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].Command != "git" {
		t.Errorf("expected command 'git', got %q", entries[0].Command)
	}
	if entries[0].Status != history.StatusSuccess {
		t.Errorf("expected SUCCESS, got %q", entries[0].Status)
	}
}

func TestExecutor_RecordsFailure(t *testing.T) {
	repoDir, cleanup := setupTestRepo(t)
	defer cleanup()

	recorder := history.NewRecorder()
	exec := NewExecutor(recorder, false)
	ctx := context.Background()

	// Run a command that fails
	_, _ = exec.Run(ctx, repoDir, "switch", "nonexistent-branch-xyz")

	entries := recorder.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].Status != history.StatusFailed {
		t.Errorf("expected FAILED, got %q", entries[0].Status)
	}
}
