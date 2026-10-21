//go:build !windows

package cli

// enableWindowsANSI is a no-op on non-Windows platforms.
func enableWindowsANSI() {}
