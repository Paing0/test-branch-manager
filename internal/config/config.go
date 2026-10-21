// Package config provides configuration types for the branch management workflow.
package config

import "time"

// Config holds the application configuration for a single workflow run.
type Config struct {
	// Verbose enables detailed Git command output.
	Verbose bool

	// LogFile is the optional path to write a sanitized command history.
	LogFile string

	// WorkDir is the directory to scan for Git repositories.
	WorkDir string

	// Remote is the Git remote name (default "origin").
	Remote string

	// DateStr is the formatted date string (YYYYMMDD) captured once at workflow start.
	DateStr string

	// Date is the captured time at workflow start.
	Date time.Time
}

// New creates a new Config with sensible defaults.
// The date is captured once so both branch names use the same date.
func New(workDir string, verbose bool, logFile string) *Config {
	now := time.Now()
	return &Config{
		Verbose: verbose,
		LogFile: logFile,
		WorkDir: workDir,
		Remote:  "origin",
		DateStr: now.Format("20060102"),
		Date:    now,
	}
}

// SetDate updates the date and formatted date string.
func (c *Config) SetDate(t time.Time) {
	c.Date = t
	c.DateStr = t.Format("20060102")
}

// OffshoreBranchName returns the offshore test branch name for this run.
func (c *Config) OffshoreBranchName() string {
	return "test/offshore-test-" + c.DateStr
}

// ITBranchName returns the IT test branch name for this run.
func (c *Config) ITBranchName() string {
	return "test/it-" + c.DateStr
}
