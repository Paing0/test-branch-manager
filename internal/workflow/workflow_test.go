package workflow

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"test-branch-creator/internal/cli"
	"test-branch-creator/internal/config"
	"test-branch-creator/internal/history"
)

// setupIntegrationRepo creates a complete test environment with:
// - A bare remote repository
// - A working repository with origin pointing to the bare repo
// - A develop branch with at least one commit
// - A feature branch with non-conflicting changes
// Returns: workDir (parent of repos), repoDir, remoteDir, cleanup
func setupIntegrationRepo(t *testing.T) (string, string, string, func()) {
	t.Helper()

	workDir, err := os.MkdirTemp("", "integration-*")
	if err != nil {
		t.Fatal(err)
	}

	remoteDir := filepath.Join(workDir, "remote.git")
	repoDir := filepath.Join(workDir, "my-service")

	os.MkdirAll(remoteDir, 0o755)
	os.MkdirAll(repoDir, 0o755)

	cleanup := func() {
		os.RemoveAll(workDir)
	}

	// Init bare remote
	gitRun(t, remoteDir, "init", "--bare")

	// Init working repo
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.email", "test@test.com")
	gitRun(t, repoDir, "config", "user.name", "Test User")
	gitRun(t, repoDir, "remote", "add", "origin", remoteDir)

	// Initial commit on main
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "# My Service\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "initial commit")
	gitRun(t, repoDir, "push", "origin", "HEAD")

	// Create develop branch
	gitRun(t, repoDir, "switch", "-c", "develop")
	writeTestFile(t, filepath.Join(repoDir, "develop.txt"), "develop baseline\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "develop baseline")
	gitRun(t, repoDir, "push", "origin", "develop")

	// Create feature branch
	gitRun(t, repoDir, "switch", "-c", "feature/payment-api")
	writeTestFile(t, filepath.Join(repoDir, "payment.go"), "package payment\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "add payment API")
	gitRun(t, repoDir, "push", "origin", "feature/payment-api")

	// Go back to develop
	gitRun(t, repoDir, "switch", "develop")

	return workDir, repoDir, remoteDir, cleanup
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed in %s: %v\nOutput: %s", strings.Join(args, " "), dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFullWorkflow_Success(t *testing.T) {
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	// Mock prompter that selects repo, date, feature branch, says no to IT merge, confirms pushes
	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			false, // merge existing IT test branch? No
			true,  // Push offshore branch?
			true,  // Push IT branch?
		},
		SelectResponses: []string{
			repoDir,                      // Select repository
			"20261021",                   // Select date (today)
			"origin/feature/payment-api", // Select source branch
		},
	}

	cfg := &config.Config{
		Verbose: false,
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err != nil {
		t.Fatalf("workflow failed: %v\nOutput:\n%s", err, output.String())
	}

	if recorder.Workflow.Status != "Completed successfully" {
		t.Errorf("expected status 'Completed successfully', got %q", recorder.Workflow.Status)
	}

	// Verify branches were created
	branches := gitRun(t, repoDir, "branch", "-a")
	if !strings.Contains(branches, "test/offshore-test-20261021") {
		t.Errorf("offshore branch not found in: %s", branches)
	}
	if !strings.Contains(branches, "test/it-20261021") {
		t.Errorf("IT branch not found in: %s", branches)
	}

	// Verify branches exist on remote
	remoteBranches := gitRun(t, repoDir, "branch", "-r")
	if !strings.Contains(remoteBranches, "origin/test/offshore-test-20261021") {
		t.Errorf("offshore branch not on remote: %s", remoteBranches)
	}
	if !strings.Contains(remoteBranches, "origin/test/it-20261021") {
		t.Errorf("IT branch not on remote: %s", remoteBranches)
	}

	// Verify command history
	entries := recorder.Entries()
	if len(entries) == 0 {
		t.Error("expected some command history entries")
	}

	// Verify critical push operations succeeded
	hasSuccessfulPush := false
	for _, e := range entries {
		if len(e.Args) > 0 && e.Args[0] == "push" && e.Status == history.StatusSuccess {
			hasSuccessfulPush = true
		}
	}
	if !hasSuccessfulPush {
		t.Error("expected successful push in history")
	}
}

func TestFullWorkflow_WithExistingITMerge(t *testing.T) {
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	// Create an existing IT test branch
	gitRun(t, repoDir, "switch", "develop")
	gitRun(t, repoDir, "switch", "-c", "test/it-20261020")
	writeTestFile(t, filepath.Join(repoDir, "it-config.txt"), "IT config\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "existing IT branch")
	gitRun(t, repoDir, "push", "origin", "test/it-20261020")
	gitRun(t, repoDir, "switch", "develop")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			true, // merge existing IT test branch? Yes
			true, // Push offshore branch
			true, // Push IT branch
		},
		SelectResponses: []string{
			repoDir,                      // Select repo
			"20261021",                   // Select date (today)
			"origin/feature/payment-api", // Select source branch
			"origin/test/it-20261020",    // Select existing IT branch
		},
	}

	cfg := &config.Config{
		Verbose: false,
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err != nil {
		t.Fatalf("workflow failed: %v\nOutput:\n%s", err, output.String())
	}

	if recorder.Workflow.Status != "Completed successfully" {
		t.Errorf("expected 'Completed successfully', got %q", recorder.Workflow.Status)
	}
}

func TestWorkflow_MergeConflict(t *testing.T) {
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	// Create conflicting changes
	// 1. Branch feature/conflicting from current develop
	gitRun(t, repoDir, "switch", "develop")
	gitRun(t, repoDir, "switch", "-c", "feature/conflicting")
	writeTestFile(t, filepath.Join(repoDir, "shared.txt"), "feature version\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "conflicting change")
	gitRun(t, repoDir, "push", "origin", "feature/conflicting")

	// 2. Add conflicting change directly to develop
	gitRun(t, repoDir, "switch", "develop")
	writeTestFile(t, filepath.Join(repoDir, "shared.txt"), "develop version\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "develop change")
	gitRun(t, repoDir, "push", "origin", "develop")

	gitRun(t, repoDir, "switch", "develop")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	// Prompter that cancels at conflict resolution
	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			false, // merge existing IT test branch? No
		},
		SelectResponses: []string{
			repoDir,                      // Select repo
			"20261021",                   // Select date (today)
			"origin/feature/conflicting", // Select source branch
		},
		WaitResponses: []error{
			cli.ErrUserCancelled, // Cancel at conflict resolution
		},
	}

	cfg := &config.Config{
		Verbose: false,
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected workflow to fail due to conflicts")
	}

	if !strings.Contains(recorder.Workflow.Status, "conflict") &&
		!strings.Contains(recorder.Workflow.Status, "Phase B") {
		t.Errorf("expected status to mention conflict or Phase B, got %q", recorder.Workflow.Status)
	}

	// Verify recovery messages exist
	if len(recorder.Workflow.RecoveryMessages) == 0 {
		t.Error("expected recovery messages")
	}
}

func TestWorkflow_UserCancellation(t *testing.T) {
	workDir, _, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	// Prompter that cancels at repo selection
	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			false, // Decline the repository
		},
		SelectResponses: []string{}, // No selections
	}

	cfg := &config.Config{
		Verbose: false,
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected workflow to fail due to cancellation")
	}

	if !strings.Contains(recorder.Workflow.Status, "Cancel") {
		t.Errorf("expected status to mention cancellation, got %q", recorder.Workflow.Status)
	}
}

func TestWorkflow_NoRepositories(t *testing.T) {
	emptyDir, err := os.MkdirTemp("", "empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(emptyDir)

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{}

	cfg := &config.Config{
		WorkDir: emptyDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err = wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected error when no repositories found")
	}

	if !strings.Contains(err.Error(), "no Git repositories") {
		t.Errorf("expected 'no Git repositories' error, got: %v", err)
	}
}

func TestWorkflow_MissingDevelop(t *testing.T) {
	workDir, err := os.MkdirTemp("", "no-develop-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	// Create a repo without a develop branch
	repoDir := filepath.Join(workDir, "repo")
	remoteDir := filepath.Join(workDir, "remote.git")
	os.MkdirAll(repoDir, 0o755)
	os.MkdirAll(remoteDir, 0o755)

	gitRun(t, remoteDir, "init", "--bare")
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.email", "test@test.com")
	gitRun(t, repoDir, "config", "user.name", "Test User")
	gitRun(t, repoDir, "remote", "add", "origin", remoteDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "# Test\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "initial")
	gitRun(t, repoDir, "push", "origin", "HEAD")

	// Create a feature branch (so there's something to select)
	gitRun(t, repoDir, "switch", "-c", "feature/test")
	gitRun(t, repoDir, "push", "origin", "feature/test")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{
		SelectResponses: []string{
			repoDir,              // Select repo
			"20261021",           // Select date (today)
			"origin/feature/test", // Select source branch
		},
	}

	cfg := &config.Config{
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err = wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected error for missing develop branch")
	}
	if !strings.Contains(err.Error(), "develop") {
		t.Errorf("expected error to mention 'develop', got: %v", err)
	}
}

func TestWorkflow_BranchAlreadyExists(t *testing.T) {
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	// Create the offshore branch already
	gitRun(t, repoDir, "switch", "-c", "test/offshore-test-20261021")
	gitRun(t, repoDir, "push", "origin", "test/offshore-test-20261021")
	gitRun(t, repoDir, "switch", "develop")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			true, // Branch exists - cancel? Yes
		},
		SelectResponses: []string{
			repoDir,                      // Select repo
			"20261021",                   // Select date (today)
			"origin/feature/payment-api", // Select source branch
		},
	}

	cfg := &config.Config{
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected error when branch already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestWorkflow_CommandHistoryOnFailure(t *testing.T) {
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			false, // merge existing IT? No
			true,  // Push offshore? Yes
			true,  // Push IT? Yes
		},
		SelectResponses: []string{
			repoDir,
			"20261021", // Select date (today)
			"origin/feature/payment-api",
		},
	}

	cfg := &config.Config{
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)

	// Don't actually break anything — let the workflow run, then check history
	err := wf.Run(context.Background())

	// Whether it succeeds or fails, verify history was recorded
	entries := recorder.Entries()
	if len(entries) == 0 {
		t.Error("expected command history to be recorded")
	}

	// Verify summary can be printed
	var summaryBuf bytes.Buffer
	recorder.PrintSummary(&summaryBuf)
	summary := summaryBuf.String()

	if !strings.Contains(summary, "Workflow Summary") {
		t.Errorf("expected 'Workflow Summary' in output:\n%s", summary)
	}
	if !strings.Contains(summary, "Git Commands Executed") {
		t.Errorf("expected 'Git Commands Executed' in output:\n%s", summary)
	}

	_ = err // We don't care about the error here, just the history
}

func TestWorkflow_MissingRemote(t *testing.T) {
	workDir, err := os.MkdirTemp("", "no-remote-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	// Create repo without any remote
	repoDir := filepath.Join(workDir, "repo")
	os.MkdirAll(repoDir, 0o755)
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.email", "test@test.com")
	gitRun(t, repoDir, "config", "user.name", "Test User")
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "# Test\n")
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "commit", "-m", "initial")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	prompter := &cli.MockPrompter{
		SelectResponses: []string{
			repoDir,    // Select repo
			"20261021", // Select date (today)
		},
	}

	cfg := &config.Config{
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err = wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected error for missing remote")
	}
	if !strings.Contains(err.Error(), "remote") {
		t.Errorf("expected error to mention 'remote', got: %v", err)
	}
}

func TestWorkflow_ResumeDetection(t *testing.T) {
	// Test that the workflow detects existing branches for the same date
	workDir, repoDir, _, cleanup := setupIntegrationRepo(t)
	defer cleanup()

	// Pre-create the offshore branch (simulate a previous partial run)
	gitRun(t, repoDir, "switch", "-c", "test/offshore-test-20261021")
	gitRun(t, repoDir, "switch", "develop")

	recorder := history.NewRecorder()
	var output bytes.Buffer
	ui := cli.NewUIWithWriter(&output)

	// User says yes to cancel when branch exists
	prompter := &cli.MockPrompter{
		ConfirmResponses: []bool{
			true, // Branch exists, cancel? Yes
		},
		SelectResponses: []string{
			repoDir,
			"20261021", // Select date (today)
			"origin/feature/payment-api",
		},
	}

	cfg := &config.Config{
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: "20261021",
	}

	wf := New(cfg, ui, prompter, recorder)
	err := wf.Run(context.Background())

	if err == nil {
		t.Fatal("expected error when branch already exists")
	}

	out := output.String()
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected warning about existing branch in output:\n%s", out)
	}
}
