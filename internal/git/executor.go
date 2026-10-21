// Package git provides Git command execution and repository operations.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"test-branch-creator/internal/history"
)

// Executor runs Git commands and records them to the history recorder.
type Executor struct {
	recorder *history.Recorder
	verbose  bool
}

// NewExecutor creates a new Git command executor.
func NewExecutor(recorder *history.Recorder, verbose bool) *Executor {
	return &Executor{
		recorder: recorder,
		verbose:  verbose,
	}
}

// Result holds the output of a Git command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a Git command with the given arguments in the specified directory.
// It records the command in the history recorder and returns the result.
func (e *Executor) Run(ctx context.Context, workDir string, args ...string) (*Result, error) {
	idx := e.recorder.Record("git", args, workDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &Result{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			e.recorder.Complete(idx, result.ExitCode, fmt.Errorf("%s", result.Stderr))
		} else {
			result.ExitCode = -1
			e.recorder.Complete(idx, -1, err)
		}
		return result, err
	}

	e.recorder.Complete(idx, 0, nil)
	return result, nil
}

// RunAllowFailure executes a Git command that is expected to potentially fail.
// Returns the result and exit code without wrapping the error.
func (e *Executor) RunAllowFailure(ctx context.Context, workDir string, args ...string) (*Result, error) {
	return e.Run(ctx, workDir, args...)
}

// Recorder returns the underlying history recorder.
func (e *Executor) Recorder() *history.Recorder {
	return e.recorder
}
