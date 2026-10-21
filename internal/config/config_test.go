package config

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := New("/some/dir", true, "test.log")

	if cfg.WorkDir != "/some/dir" {
		t.Errorf("expected WorkDir '/some/dir', got %q", cfg.WorkDir)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose to be true")
	}
	if cfg.LogFile != "test.log" {
		t.Errorf("expected LogFile 'test.log', got %q", cfg.LogFile)
	}
	if cfg.Remote != "origin" {
		t.Errorf("expected Remote 'origin', got %q", cfg.Remote)
	}
	if cfg.DateStr == "" {
		t.Error("expected DateStr to be set")
	}
	if len(cfg.DateStr) != 8 {
		t.Errorf("expected DateStr length 8 (YYYYMMDD), got %d: %q", len(cfg.DateStr), cfg.DateStr)
	}
}

func TestSetDate(t *testing.T) {
	cfg := New("/some/dir", false, "")
	target := time.Date(2026, 10, 22, 10, 0, 0, 0, time.Local)
	cfg.SetDate(target)

	if cfg.DateStr != "20261022" {
		t.Errorf("expected DateStr '20261022', got %q", cfg.DateStr)
	}
	if cfg.OffshoreBranchName() != "test/offshore-test-20261022" {
		t.Errorf("expected offshore branch 'test/offshore-test-20261022', got %q", cfg.OffshoreBranchName())
	}
	if cfg.ITBranchName() != "test/it-20261022" {
		t.Errorf("expected IT branch 'test/it-20261022', got %q", cfg.ITBranchName())
	}
}

func TestBranchNames(t *testing.T) {
	cfg := &Config{
		DateStr: "20261021",
		Date:    time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local),
	}

	tests := []struct {
		name     string
		fn       func() string
		expected string
	}{
		{
			name:     "OffshoreBranchName",
			fn:       cfg.OffshoreBranchName,
			expected: "test/offshore-test-20261021",
		},
		{
			name:     "ITBranchName",
			fn:       cfg.ITBranchName,
			expected: "test/it-20261021",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestDateFormat(t *testing.T) {
	// Verify that the date format produces YYYYMMDD
	tests := []struct {
		time     time.Time
		expected string
	}{
		{time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), "20260105"},
		{time.Date(2026, 10, 21, 15, 30, 0, 0, time.UTC), "20261021"},
		{time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), "20261231"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.time.Format("20060102")
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBranchNameConsistency(t *testing.T) {
	// Both branch names must use the same date
	cfg := New("/test", false, "")
	offshore := cfg.OffshoreBranchName()
	it := cfg.ITBranchName()

	// Extract dates from branch names
	offshoreDate := offshore[len("test/offshore-test-"):]
	itDate := it[len("test/it-"):]

	if offshoreDate != itDate {
		t.Errorf("offshore date %q != IT date %q", offshoreDate, itDate)
	}
	if offshoreDate != cfg.DateStr {
		t.Errorf("branch date %q != config date %q", offshoreDate, cfg.DateStr)
	}
}
