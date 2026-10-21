// Package history provides a thread-safe recorder for Git command execution history.
package history

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Status represents the outcome of a Git command.
type Status string

const (
	StatusSuccess     Status = "SUCCESS"
	StatusFailed      Status = "FAILED"
	StatusInterrupted Status = "INTERRUPTED"
)

// Entry records a single Git command execution.
type Entry struct {
	Sequence  int
	Command   string
	Args      []string
	WorkDir   string
	StartTime time.Time
	Duration  time.Duration
	ExitCode  int
	Status    Status
	Error     string
}

// CommandString returns the full command as a display string.
func (e *Entry) CommandString() string {
	parts := []string{e.Command}
	parts = append(parts, e.Args...)
	return strings.Join(parts, " ")
}

// WorkflowState captures the overall workflow status for the summary.
type WorkflowState struct {
	Status           string
	Repository       string
	RepositoryPath   string
	SourceBranch     string
	OffshoreBranch   string
	ITBranch         string
	Remote           string
	RecoveryMessages []string
}

// Recorder tracks all Git commands executed during a workflow run.
type Recorder struct {
	mu       sync.Mutex
	entries  []Entry
	seq      int
	Workflow WorkflowState
}

// NewRecorder creates a new empty Recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		Workflow: WorkflowState{
			Status: "Not started",
		},
	}
}

// Record creates a new entry, increments the sequence, and returns the entry index.
func (r *Recorder) Record(command string, args []string, workDir string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	entry := Entry{
		Sequence:  r.seq,
		Command:   command,
		Args:      args,
		WorkDir:   workDir,
		StartTime: time.Now(),
		Status:    StatusInterrupted, // default until completed
	}
	r.entries = append(r.entries, entry)
	return len(r.entries) - 1
}

// Complete updates an entry with the final status.
func (r *Recorder) Complete(index int, exitCode int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= len(r.entries) {
		return
	}
	e := &r.entries[index]
	e.Duration = time.Since(e.StartTime)
	e.ExitCode = exitCode
	if err != nil {
		e.Status = StatusFailed
		e.Error = err.Error()
	} else if exitCode != 0 {
		e.Status = StatusFailed
	} else {
		e.Status = StatusSuccess
	}
}

// Entries returns a copy of all recorded entries.
func (r *Recorder) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Entry, len(r.entries))
	copy(result, r.entries)
	return result
}

// Count returns the number of recorded commands.
func (r *Recorder) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// PrintSummary writes the workflow summary and command history to the given writer.
func (r *Recorder) PrintSummary(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Workflow Summary ===")
	fmt.Fprintf(w, "Status: %s\n", r.Workflow.Status)

	if r.Workflow.Repository != "" {
		fmt.Fprintf(w, "Repository: %s (%s)\n", r.Workflow.Repository, r.Workflow.RepositoryPath)
	}
	if r.Workflow.SourceBranch != "" {
		fmt.Fprintf(w, "Source branch: %s\n", r.Workflow.SourceBranch)
	}
	if r.Workflow.OffshoreBranch != "" {
		fmt.Fprintf(w, "Offshore branch: %s\n", r.Workflow.OffshoreBranch)
	} else {
		fmt.Fprintln(w, "Offshore branch: Not created")
	}
	if r.Workflow.ITBranch != "" {
		fmt.Fprintf(w, "IT test branch: %s\n", r.Workflow.ITBranch)
	} else {
		fmt.Fprintln(w, "IT test branch: Not created")
	}
	if r.Workflow.Remote != "" {
		fmt.Fprintf(w, "Remote: %s\n", r.Workflow.Remote)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "=== Git Commands Executed ===")

	if len(r.entries) == 0 {
		fmt.Fprintln(w, "(No Git commands were executed)")
	} else {
		for _, e := range r.entries {
			fmt.Fprintf(w, "[%d] %s\n", e.Sequence, e.CommandString())
			fmt.Fprintf(w, "    Directory: %s\n", e.WorkDir)
			fmt.Fprintf(w, "    Status: %s", e.Status)
			if e.Duration > 0 {
				fmt.Fprintf(w, " (%.1fs)", e.Duration.Seconds())
			}
			fmt.Fprintln(w)
			if e.Error != "" {
				fmt.Fprintf(w, "    Reason: %s\n", e.Error)
			}
			fmt.Fprintln(w)
		}
	}

	if len(r.Workflow.RecoveryMessages) > 0 {
		fmt.Fprintln(w, "=== Recovery Instructions ===")
		for i, msg := range r.Workflow.RecoveryMessages {
			fmt.Fprintf(w, "%d. %s\n", i+1, msg)
		}
		fmt.Fprintln(w)
	}
}

// WriteToFile writes a sanitized summary to a log file.
func (r *Recorder) WriteToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "Git Branch Manager - Command Log\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	r.PrintSummary(f)
	return nil
}
