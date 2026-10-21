// Package cli provides terminal UI utilities for colored output and interactive prompts.
// This implementation uses only the Go standard library for maximum portability.
package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// ANSI color codes
const (
	colorReset     = "\033[0m"
	colorBold      = "\033[1m"
	colorUnderline = "\033[4m"
	colorDim       = "\033[2m"
	colorRed       = "\033[31m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
	colorBlue      = "\033[34m"
	colorMagenta   = "\033[35m"
	colorCyan      = "\033[36m"
	colorGray      = "\033[90m"
	colorBoldRed   = "\033[1;31m"
)

// UI provides styled terminal output.
type UI struct {
	writer   io.Writer
	useColor bool
}

// NewUI creates a new UI instance with color auto-detection.
func NewUI() *UI {
	return &UI{
		writer:   os.Stdout,
		useColor: supportsColor(),
	}
}

// NewUIWithWriter creates a UI that writes to the given writer with no color. Useful for testing.
func NewUIWithWriter(w io.Writer) *UI {
	return &UI{
		writer:   w,
		useColor: false,
	}
}

// supportsColor checks if the terminal likely supports ANSI colors.
func supportsColor() bool {
	// Check NO_COLOR environment variable (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	// Check TERM
	term := os.Getenv("TERM")
	if term == "dumb" {
		return false
	}
	// On Windows, modern terminals support ANSI but we enable virtual terminal processing
	if runtime.GOOS == "windows" {
		enableWindowsANSI()
		return true
	}
	return true
}

// color wraps text in ANSI color codes if color is enabled.
func (u *UI) color(code string, text string) string {
	if !u.useColor {
		return text
	}
	return code + text + colorReset
}

// Info prints an informational message.
func (u *UI) Info(msg string) {
	prefix := u.color(colorBlue, "i")
	fmt.Fprintf(u.writer, "%s %s\n", prefix, msg)
}

// Infof prints a formatted informational message.
func (u *UI) Infof(format string, a ...interface{}) {
	u.Info(fmt.Sprintf(format, a...))
}

// Success prints a success message.
func (u *UI) Success(msg string) {
	prefix := u.color(colorGreen, "OK")
	fmt.Fprintf(u.writer, "%s %s\n", prefix, msg)
}

// Successf prints a formatted success message.
func (u *UI) Successf(format string, a ...interface{}) {
	u.Success(fmt.Sprintf(format, a...))
}

// Warning prints a warning message.
func (u *UI) Warning(msg string) {
	prefix := u.color(colorYellow, "WARNING")
	fmt.Fprintf(u.writer, "%s %s\n", prefix, msg)
}

// Warningf prints a formatted warning message.
func (u *UI) Warningf(format string, a ...interface{}) {
	u.Warning(fmt.Sprintf(format, a...))
}

// Error prints an error message.
func (u *UI) Error(msg string) {
	prefix := u.color(colorRed, "ERROR")
	fmt.Fprintf(u.writer, "%s %s\n", prefix, msg)
}

// Errorf prints a formatted error message.
func (u *UI) Errorf(format string, a ...interface{}) {
	u.Error(fmt.Sprintf(format, a...))
}

// Danger prints a message for potentially destructive operations.
func (u *UI) Danger(msg string) {
	fmt.Fprintf(u.writer, "%s\n", u.color(colorBoldRed, "!! "+msg))
}

// Header prints a section header.
func (u *UI) Header(msg string) {
	line := strings.Repeat("=", len(msg)+4)
	fmt.Fprintf(u.writer, "\n%s\n", u.color(colorBold+colorUnderline, msg))
	if !u.useColor {
		fmt.Fprintf(u.writer, "%s\n", line)
	}
	fmt.Fprintln(u.writer)
}

// Step prints a workflow step indicator.
func (u *UI) Step(phase string, description string) {
	phaseStr := u.color(colorCyan+colorBold, fmt.Sprintf("[%s]", phase))
	fmt.Fprintf(u.writer, "%s %s\n", phaseStr, description)
}

// BranchName formats a branch name for display.
func (u *UI) BranchName(name string) string {
	return u.color(colorMagenta, name)
}

// Dim formats text in a dimmed style.
func (u *UI) Dim(msg string) string {
	return u.color(colorGray, msg)
}

// FileList prints a list of file paths.
func (u *UI) FileList(title string, files []string) {
	fmt.Fprintf(u.writer, "\n%s\n", u.color(colorYellow, title))
	for _, f := range files {
		fmt.Fprintf(u.writer, "  - %s\n", f)
	}
	fmt.Fprintln(u.writer)
}

// Progress prints a progress indicator.
func (u *UI) Progress(msg string) {
	fmt.Fprintf(u.writer, "%s %s...\n", u.color(colorGray, "->"), msg)
}

// Blank prints an empty line.
func (u *UI) Blank() {
	fmt.Fprintln(u.writer)
}

// Print writes a plain message.
func (u *UI) Print(msg string) {
	fmt.Fprintln(u.writer, msg)
}

// Printf writes a formatted plain message.
func (u *UI) Printf(format string, a ...interface{}) {
	fmt.Fprintf(u.writer, format, a...)
}

// SummaryLine prints a key-value summary line.
func (u *UI) SummaryLine(key string, value string) {
	label := u.color(colorGray, key+":")
	fmt.Fprintf(u.writer, "  %s %s\n", label, value)
}
