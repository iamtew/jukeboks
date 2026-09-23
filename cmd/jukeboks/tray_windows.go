//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	wmDestroy     = 0x0002
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmContextMenu = 0x007B
	wmApp         = 0x8000
	wmTray        = wmApp + 1
	ninSelect     = 0x0400

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 1
	nifIcon    = 2
	nifTip     = 4

	idiApplication = 32512
	idcArrow       = 32512
	mbOK           = 0
	mbIconError    = 0x10
	swShownormal   = 1

	mfString       = 0
	mfSeparator    = 0x800
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	idOpenHome    = 1001
	idOpenAdmin   = 1002
	idOpenOverlay = 1003
	idRestart     = 1004
	idQuit        = 1005

	attachParentProcess = ^uintptr(0)
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procLoadCursorW      = user32.NewProc("LoadCursorW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procMessageBoxW      = user32.NewProc("MessageBoxW")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procCreateMutexW     = kernel32.NewProc("CreateMutexW")
	procAttachConsole    = kernel32.NewProc("AttachConsole")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW    = shell32.NewProc("ShellExecuteW")
	procExtractIconExW   = shell32.NewProc("ExtractIconExW")

	trayWndProc = syscall.NewCallback(trayProc)
	trayHwnd    syscall.Handle
	trayIcon    notifyIconData
	menuOpen    bool
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   syscall.Handle
	Icon       syscall.Handle
	Cursor     syscall.Handle
	Background syscall.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     syscall.Handle
}

type notifyIconData struct {
	Size            uint32
	Wnd             syscall.Handle
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            syscall.Handle
	Tip             [128]uint16
}

type point struct {
	X, Y int32
}

type msg struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

func attachParentConsole() {
	r, _, _ := procAttachConsole.Call(attachParentProcess)
	if r == 0 {
		return
	}
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil || h == 0 || h == syscall.InvalidHandle {
		return
	}
	f := os.NewFile(uintptr(h), "CONOUT$")
	os.Stdout = f
	os.Stderr = f
}

func alert(title, msgText string) {
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16(msgText))), uintptr(unsafe.Pointer(utf16(title))), mbOK|mbIconError)
}

func fatalf(format string, args ...any) {
	alert("Jukeboks", fmt.Sprintf(format, args...))
	os.Exit(1)
}

func waitForQuit() {
	_, _, muErr := procCreateMutexW.Call(0, 1, uintptr(unsafe.Pointer(utf16("Local\\JukeboksSingleInstance"))))
	if muErr == syscall.ERROR_ALREADY_EXISTS {
		fatalf("Jukeboks is already running")
	}

	instance, _, _ := procGetModuleHandleW.Call(0)
	hInstance := syscall.Handle(instance)
	hIcon := loadAppIcon(hInstance)
	hCursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	className := utf16("JukeboksTray")
	wc := wndClassEx{
		WndProc:   trayWndProc,
		Instance:  hInstance,
		Icon:      syscall.Handle(hIcon),
		Cursor:    syscall.Handle(hCursor),
		IconSm:    syscall.Handle(hIcon),
		ClassName: className,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16("Jukeboks"))), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		fatalf("failed to create tray window: %v", err)
	}
	trayHwnd = syscall.Handle(hwnd)

	trayIcon = notifyIconData{
		Wnd:             trayHwnd,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmTray,
		Icon:            syscall.Handle(hIcon),
	}
	trayIcon.Size = uint32(unsafe.Sizeof(trayIcon))
	copyUTF16(trayIcon.Tip[:], "Jukeboks")
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayIcon))); r == 0 {
		fatalf("failed to add tray icon: %v", err)
	}

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayIcon)))
	stopHTTP()
}

func trayProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmTray:
		switch lParam {
		case wmLButtonUp, ninSelect, wmRButtonUp, wmContextMenu:
			showMenu(syscall.Handle(hwnd))
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return ret
}

func showMenu(hwnd syscall.Handle) {
	if menuOpen {
		return
	}
	menuOpen = true
	defer func() { menuOpen = false }()

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	addMenu(menu, idOpenHome, "Open Home")
	addMenu(menu, idOpenAdmin, "Open Admin")
	addMenu(menu, idOpenOverlay, "Open Overlay")
	procAppendMenuW.Call(menu, mfSeparator, 0, 0)
	addMenu(menu, idRestart, "Restart")
	addMenu(menu, idQuit, "Quit")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(uintptr(hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	procPostMessageW.Call(uintptr(hwnd), 0, 0, 0)

	switch cmd {
	case idOpenHome:
		openURL(servingURL("/"))
	case idOpenAdmin:
		openURL(servingURL("/admin/"))
	case idOpenOverlay:
		openURL(servingURL("/overlay/"))
	case idRestart:
		restartHTTP()
	case idQuit:
		procPostQuitMessage.Call(0)
	}
}

func addMenu(menu uintptr, id uintptr, label string) {
	procAppendMenuW.Call(menu, mfString, id, uintptr(unsafe.Pointer(utf16(label))))
}

func loadAppIcon(instance syscall.Handle) syscall.Handle {
	if exe, err := os.Executable(); err == nil {
		var large, small syscall.Handle
		n, _, _ := procExtractIconExW.Call(
			uintptr(unsafe.Pointer(utf16(exe))),
			0,
			uintptr(unsafe.Pointer(&large)),
			uintptr(unsafe.Pointer(&small)),
			1,
		)
		if n > 0 && large != 0 {
			return large
		}
		if small != 0 {
			return small
		}
	}
	if h, _, _ := procLoadIconW.Call(uintptr(instance), 1); h != 0 {
		return syscall.Handle(h)
	}
	h, _, _ := procLoadIconW.Call(0, idiApplication)
	return syscall.Handle(h)
}

func openURL(u string) {
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16("open"))), uintptr(unsafe.Pointer(utf16(u))), 0, 0, swShownormal)
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func copyUTF16(dst []uint16, s string) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	n := copy(dst, u)
	if n < len(dst) {
		dst[n] = 0
	} else {
		dst[len(dst)-1] = 0
	}
}
