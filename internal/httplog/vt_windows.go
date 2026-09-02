//go:build windows

package httplog

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleOutputCP         = kernel32.NewProc("SetConsoleOutputCP")
)

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

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

func terminalSize() (width, height int, ok bool) {
	var info consoleScreenBufferInfo
	if r, _, _ := procGetConsoleScreenBufferInfo.Call(os.Stderr.Fd(), uintptr(unsafe.Pointer(&info))); r == 0 {
		return 0, 0, false
	}

	width = int(info.window.right-info.window.left) + 1
	height = int(info.window.bottom-info.window.top) + 1
	if width < 1 || height < 2 {
		return 0, 0, false
	}
	return width, height, true
}
