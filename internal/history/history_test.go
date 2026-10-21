package history

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewRecorder(t *testing.T) {
	r := NewRecorder()
	if r.Count() != 0 {
		t.Errorf("expected 0 entries, got %d", r.Count())
	}
	if r.Workflow.Status != "Not started" {
		t.Errorf("expected status 'Not started', got %q", r.Workflow.Status)
	}
}

func TestRecordAndComplete(t *testing.T) {
	r := NewRecorder()

	idx := r.Record("git", []string{"fetch", "origin"}, "/test/repo")
	if r.Count() != 1 {
		t.Fatalf("expected 1 entry, got %d", r.Count())
	}

	entries := r.Entries()
	if entries[0].Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", entries[0].Sequence)
	}
	if entries[0].Command != "git" {
		t.Errorf("expected command 'git', got %q", entries[0].Command)
	}
	if entries[0].Status != StatusInterrupted {
		t.Errorf("expected initial status INTERRUPTED, got %q", entries[0].Status)
	}

	// Complete successfully
	r.Complete(idx, 0, nil)
	entries = r.Entries()
	if entries[0].Status != StatusSuccess {
		t.Errorf("expected status SUCCESS, got %q", entries[0].Status)
	}
}

func TestRecordMultiple(t *testing.T) {
	r := NewRecorder()

	r.Record("git", []string{"fetch", "origin"}, "/repo")
	r.Complete(0, 0, nil)

	r.Record("git", []string{"switch", "-c", "test/branch"}, "/repo")
	r.Complete(1, 0, nil)

	idx := r.Record("git", []string{"merge", "origin/feature"}, "/repo")
	r.Complete(idx, 1, nil)

	if r.Count() != 3 {
		t.Fatalf("expected 3 entries, got %d", r.Count())
	}

	entries := r.Entries()
	if entries[0].Sequence != 1 {
		t.Errorf("expected first sequence 1, got %d", entries[0].Sequence)
	}
	if entries[2].Sequence != 3 {
		t.Errorf("expected third sequence 3, got %d", entries[2].Sequence)
	}
	if entries[2].Status != StatusFailed {
		t.Errorf("expected third status FAILED, got %q", entries[2].Status)
	}
}

func TestEntryCommandString(t *testing.T) {
	tests := []struct {
		name     string
		entry    Entry
		expected string
	}{
		{
			name:     "simple fetch",
			entry:    Entry{Command: "git", Args: []string{"fetch", "origin"}},
			expected: "git fetch origin",
		},
		{
			name:     "create branch",
			entry:    Entry{Command: "git", Args: []string{"switch", "-c", "test/branch", "origin/develop"}},
			expected: "git switch -c test/branch origin/develop",
		},
		{
			name:     "no args",
			entry:    Entry{Command: "git", Args: nil},
			expected: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.CommandString()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestPrintSummary(t *testing.T) {
	r := NewRecorder()
	r.Workflow.Status = "Completed successfully"
	r.Workflow.Repository = "my-repo"
	r.Workflow.RepositoryPath = "/path/to/my-repo"
	r.Workflow.SourceBranch = "origin/feature/test"
	r.Workflow.OffshoreBranch = "test/offshore-test-20261021"
	r.Workflow.ITBranch = "test/it-20261021"
	r.Workflow.Remote = "origin"

	idx := r.Record("git", []string{"fetch", "origin"}, "/path/to/my-repo")
	r.Complete(idx, 0, nil)

	var buf bytes.Buffer
	r.PrintSummary(&buf)

	output := buf.String()

	// Check key parts of the summary
	checks := []string{
		"Workflow Summary",
		"Completed successfully",
		"my-repo",
		"origin/feature/test",
		"test/offshore-test-20261021",
		"test/it-20261021",
		"Git Commands Executed",
		"git fetch origin",
		"SUCCESS",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("summary missing expected text %q\nFull output:\n%s", check, output)
		}
	}
}

func TestPrintSummaryEmpty(t *testing.T) {
	r := NewRecorder()
	r.Workflow.Status = "Cancelled before any commands"

	var buf bytes.Buffer
	r.PrintSummary(&buf)

	output := buf.String()

	if !strings.Contains(output, "No Git commands were executed") {
		t.Errorf("expected 'No Git commands were executed' in output:\n%s", output)
	}
	if !strings.Contains(output, "Cancelled before any commands") {
		t.Errorf("expected status in output:\n%s", output)
	}
}

func TestPrintSummaryWithRecovery(t *testing.T) {
	r := NewRecorder()
	r.Workflow.Status = "Failed"
	r.Workflow.RecoveryMessages = []string{
		"Resolve the conflicts",
		"Stage the files",
		"Rerun the application",
	}

	var buf bytes.Buffer
	r.PrintSummary(&buf)

	output := buf.String()

	if !strings.Contains(output, "Recovery Instructions") {
		t.Errorf("expected 'Recovery Instructions' in output:\n%s", output)
	}
	if !strings.Contains(output, "1. Resolve the conflicts") {
		t.Errorf("expected numbered recovery message in output:\n%s", output)
	}
}

func TestEntriesCopy(t *testing.T) {
	r := NewRecorder()
	r.Record("git", []string{"status"}, "/repo")
	r.Complete(0, 0, nil)

	entries := r.Entries()
	entries[0].Status = "MODIFIED"

	// Original should be unchanged
	original := r.Entries()
	if original[0].Status == "MODIFIED" {
		t.Error("Entries() should return a copy, not a reference")
	}
}

func TestCompleteOutOfBounds(t *testing.T) {
	r := NewRecorder()
	// Should not panic
	r.Complete(-1, 0, nil)
	r.Complete(0, 0, nil)
	r.Complete(100, 0, nil)
}

func TestRecorderThreadSafety(t *testing.T) {
	r := NewRecorder()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(n int) {
			idx := r.Record("git", []string{"cmd", string(rune('0'+n))}, "/repo")
			r.Complete(idx, 0, nil)
			_ = r.Entries()
			_ = r.Count()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if r.Count() != 10 {
		t.Errorf("expected 10 entries, got %d", r.Count())
	}
}
