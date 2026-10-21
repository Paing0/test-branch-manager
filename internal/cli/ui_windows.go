// +build windows

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

// enableWindowsANSI enables virtual terminal processing on Windows
// so ANSI escape codes work in cmd.exe and PowerShell.
func enableWindowsANSI() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")

	handle := os.Stdout.Fd()
	var mode uint32
	r, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return
	}

	// ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
	const enableVTP = 0x0004
	mode |= enableVTP
	setConsoleMode.Call(handle, uintptr(mode))
}
