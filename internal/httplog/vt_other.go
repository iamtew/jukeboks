//go:build !windows

package httplog

func enableNativeConsole() {}

func terminalSize() (width, height int, ok bool) {
	return 0, 0, false
}
