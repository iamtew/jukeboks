//go:build windows

package httplog

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
)

func enableNativeConsole() {
	_, _, _ = procSetConsoleOutputCP.Call(65001)
	enableVTForHandle(os.Stdout.Fd())
	enableVTForHandle(os.Stderr.Fd())
}

func enableVTForHandle(handle uintptr) {
	var mode uint32
	if r, _, _ := procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); r == 0 {
		return
	}
	mode |= enableVirtualTerminalProcessing
	_, _, _ = procSetConsoleMode.Call(handle, uintptr(mode))
}
