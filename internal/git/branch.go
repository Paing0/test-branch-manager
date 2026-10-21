package git

import (
	"context"
	"fmt"
	"strings"
)

// MergeResult represents the outcome of a merge operation.
type MergeResult struct {
	Success         bool
	HasConflicts    bool
	ConflictedFiles []string
	Message         string
}

// Fetch fetches the latest information from a remote.
func (r *Repository) Fetch(ctx context.Context, remote string) error {
	_, err := r.Executor.Run(ctx, r.Path, "fetch", remote)
	if err != nil {
		return fmt.Errorf("failed to fetch from %s: %w", remote, err)
	}
	return nil
}

// ListRemoteBranches returns branch names from the specified remote.
func (r *Repository) ListRemoteBranches(ctx context.Context, remote string) ([]string, error) {
	result, err := r.Executor.Run(ctx, r.Path, "branch", "-r", "--list", fmt.Sprintf("%s/*", remote))
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}
	if result.Stdout == "" {
		return nil, nil
	}

	var branches []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		branch := strings.TrimSpace(line)
		// Skip HEAD pointer
		if branch == "" || strings.Contains(branch, "->") {
			continue
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

// ListRemoteBranchesMatching returns remote branches matching a pattern.
func (r *Repository) ListRemoteBranchesMatching(ctx context.Context, remote string, pattern string) ([]string, error) {
	result, err := r.Executor.Run(ctx, r.Path, "branch", "-r", "--list", fmt.Sprintf("%s/%s", remote, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}
	if result.Stdout == "" {
		return nil, nil
	}

	var branches []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" || strings.Contains(branch, "->") {
			continue
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

// RemoteBranchExists checks if a specific branch exists on the remote.
func (r *Repository) RemoteBranchExists(ctx context.Context, remote string, branch string) (bool, error) {
	result, err := r.Executor.Run(ctx, r.Path, "ls-remote", "--heads", remote, branch)
	if err != nil {
		return false, fmt.Errorf("failed to check remote branch: %w", err)
	}
	return result.Stdout != "", nil
}

// LocalBranchExists checks if a local branch exists.
func (r *Repository) LocalBranchExists(ctx context.Context, branch string) (bool, error) {
	_, err := r.Executor.RunAllowFailure(ctx, r.Path, "rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CreateBranchFrom creates a new branch from a start point.
func (r *Repository) CreateBranchFrom(ctx context.Context, branchName string, startPoint string) error {
	_, err := r.Executor.Run(ctx, r.Path, "switch", "-c", branchName, startPoint)
	if err != nil {
		return fmt.Errorf("failed to create branch %s from %s: %w", branchName, startPoint, err)
	}
	return nil
}

// Checkout switches to the specified branch.
func (r *Repository) Checkout(ctx context.Context, branch string) error {
	_, err := r.Executor.Run(ctx, r.Path, "switch", branch)
	if err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w", branch, err)
	}
	return nil
}

// Merge attempts to merge the given branch into the current branch.
func (r *Repository) Merge(ctx context.Context, branch string) (*MergeResult, error) {
	result, err := r.Executor.RunAllowFailure(ctx, r.Path, "merge", branch)

	mergeResult := &MergeResult{}

	if err == nil {
		mergeResult.Success = true
		mergeResult.Message = result.Stdout
		return mergeResult, nil
	}

	// Check if it's a conflict
	conflictedFiles, conflictErr := r.GetConflictedFiles(ctx)
	if conflictErr == nil && len(conflictedFiles) > 0 {
		mergeResult.HasConflicts = true
		mergeResult.ConflictedFiles = conflictedFiles
		mergeResult.Message = "Merge conflicts detected"
		return mergeResult, nil
	}

	// Not a conflict — some other error
	errMsg := result.Stderr
	if errMsg == "" {
		errMsg = err.Error()
	}
	return nil, fmt.Errorf("merge failed: %s", errMsg)
}

// GetConflictedFiles returns a list of files with unresolved merge conflicts.
func (r *Repository) GetConflictedFiles(ctx context.Context) ([]string, error) {
	result, err := r.Executor.RunAllowFailure(ctx, r.Path, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		// Fall back to checking status
		return r.getConflictedFilesFromStatus(ctx)
	}
	if result.Stdout == "" {
		return nil, nil
	}

	var files []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// getConflictedFilesFromStatus uses git status to find conflicted files.
func (r *Repository) getConflictedFilesFromStatus(ctx context.Context) ([]string, error) {
	result, err := r.Executor.RunAllowFailure(ctx, r.Path, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to check status: %w", err)
	}
	if result.Stdout == "" {
		return nil, nil
	}

	var files []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if len(line) < 3 {
			continue
		}
		xy := line[:2]
		filename := strings.TrimSpace(line[3:])
		// Unmerged status codes
		if xy == "UU" || xy == "AA" || xy == "DD" ||
			xy == "AU" || xy == "UA" ||
			xy == "DU" || xy == "UD" {
			files = append(files, filename)
		}
	}
	return files, nil
}

// HasConflicts checks if there are any unresolved conflicts.
func (r *Repository) HasConflicts(ctx context.Context) (bool, error) {
	files, err := r.GetConflictedFiles(ctx)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// CommitMerge completes a merge by committing with the default merge message.
func (r *Repository) CommitMerge(ctx context.Context) error {
	_, err := r.Executor.Run(ctx, r.Path, "commit", "--no-edit")
	if err != nil {
		return fmt.Errorf("failed to complete merge commit: %w", err)
	}
	return nil
}

// Push pushes a branch to the remote.
func (r *Repository) Push(ctx context.Context, remote string, branch string) error {
	_, err := r.Executor.Run(ctx, r.Path, "push", remote, branch)
	if err != nil {
		return fmt.Errorf("failed to push %s to %s: %w", branch, remote, err)
	}
	return nil
}

// DeleteLocalBranch deletes a local branch.
func (r *Repository) DeleteLocalBranch(ctx context.Context, branch string) error {
	_, err := r.Executor.Run(ctx, r.Path, "branch", "-D", branch)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}
	return nil
}

// StageAll stages all changes for commit.
func (r *Repository) StageAll(ctx context.Context) error {
	_, err := r.Executor.Run(ctx, r.Path, "add", "-A")
	if err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}
	return nil
}

// NeedsMergeCommit checks if Git is waiting for the user to complete a merge commit.
func (r *Repository) NeedsMergeCommit(ctx context.Context) (bool, error) {
	merging, err := r.IsMergeInProgress(ctx)
	if err != nil {
		return false, err
	}
	if !merging {
		return false, nil
	}
	conflicts, err := r.HasConflicts(ctx)
	if err != nil {
		return false, err
	}
	// Merge is in progress but no conflicts remain — needs commit
	return !conflicts, nil
}

// BranchExistsAnywhere checks if a branch exists locally or on the remote.
func (r *Repository) BranchExistsAnywhere(ctx context.Context, branch string, remote string) (local bool, remoteBranch bool, err error) {
	local, err = r.LocalBranchExists(ctx, branch)
	if err != nil {
		return false, false, err
	}
	remoteBranch, err = r.RemoteBranchExists(ctx, remote, branch)
	if err != nil {
		return local, false, err
	}
	return local, remoteBranch, nil
}
