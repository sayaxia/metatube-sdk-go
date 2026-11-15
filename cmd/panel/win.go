//go:build windows

package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"github.com/metatube-community/metatube-sdk-go/internal/iconres"
	"golang.org/x/sys/windows"
)

/* ---------- Win32 plumbing ---------- */

var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetForeground     = user32.NewProc("SetForegroundWindow")
	procSendMessageW      = user32.NewProc("SendMessageW")
	procPostMessageW      = user32.NewProc("PostMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	procAppendMenuW       = user32.NewProc("AppendMenuW")
	procTrackPopupMenu    = user32.NewProc("TrackPopupMenu")
	procDestroyMenu       = user32.NewProc("DestroyMenu")
	procGetCursorPos      = user32.NewProc("GetCursorPos")
	procCreateIconFromRes = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon       = user32.NewProc("DestroyIcon")
	// DPI awareness: without these the window is bitmap-scaled on HiDPI displays.
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	shcore                            = windows.NewLazySystemDLL("shcore.dll")
	procSetProcessDpiAwareness        = shcore.NewProc("SetProcessDpiAwareness")
	procShellNotifyIconW              = shell32.NewProc("Shell_NotifyIconW")
	procMessageBoxW                   = user32.NewProc("MessageBoxW")
)

// fail reports a startup problem. A GUI build has no console, so this has to be
// a message box; the log file is only a second line of defence.
func fail(title, msg string) {
	log.Printf("%s: %s", title, msg)
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(msg)
	const mbIconError, mbTopmost = 0x10, 0x40000
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbIconError|mbTopmost)
	os.Exit(1)
}

const (
	wmNull        = 0x0000
	wmDestroy     = 0x0002
	wmSize        = 0x0005
	wmSetIcon     = 0x0080
	wmClose       = 0x0010
	wmCommand     = 0x0111
	wmSysCommand  = 0x0112
	wmContextMenu = 0x007B
	wmApp         = 0x8000

	wmLButtonUp  = 0x0202
	wmLButtonDbl = 0x0203
	wmRButtonUp  = 0x0205

	trayCallback = wmApp + 1
	wmAppQuit    = wmApp + 2

	sizeMinimized = 1
	scMinimize    = 0xF020
	scClose       = 0xF060

	swHide    = 0
	swRestore = 9

	gwlpWndProc = ^uintptr(3) // GWLP_WNDPROC (-4)
	iconSmall   = 0
	iconBig     = 1

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4
	nifInfo    = 0x10

	mfString    = 0x0
	mfGrayed    = 0x1
	mfSeparator = 0x800

	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020

	trayUID = 1

	menuShow = 101 + iota
	menuStart
	menuStop
	menuRestart
	menuExit
)

// notifyIconData matches NOTIFYICONDATAW as laid out by Windows.
type notifyIconData struct {
	CbSize           uint32
	HWnd             windows.HWND
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            windows.Handle
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutVersion  uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     windows.Handle
}

type point struct{ X, Y int32 }

/* ---------- app window ---------- */

type panelApp struct {
	cfg       *Config
	srv       *panelServer
	svc       *service
	wv        webview2.WebView
	hwnd      windows.HWND
	orig      uintptr
	icon      windows.Handle // 32px: tray icon and fallback
	iconSmall windows.Handle // 16px: WM_SETICON ICON_SMALL
	iconBig   windows.Handle // 48px: WM_SETICON ICON_BIG / taskbar
	nid       notifyIconData

	exiting atomic.Bool
	inTray  atomic.Bool
}

var currentApp *panelApp

// enableDPIAwareness tells Windows this process draws at the display's real
// pixel density. A window that does not ask for it is rendered at 96 DPI and
// then bitmap-scaled on a 125%/150% monitor, which is why the panel used to
// look softer than the same page in a browser. Must run before any window or
// WebView2 object exists.
func enableDPIAwareness() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == -4 (Windows 10 1703+)
	if r, _, _ := procSetProcessDpiAwarenessContext.Call(uintptr(0xFFFFFFFFFFFFFFFC)); r != 0 {
		return
	}
	// PROCESS_PER_MONITOR_DPI_AWARE == 2 (Windows 8.1+)
	if r, _, _ := procSetProcessDpiAwareness.Call(2); r == 0 {
		return
	}
	// PROCESS_SYSTEM_DPI_AWARE (Vista+), better than nothing.
	procSetProcessDPIAware.Call()
}

// windowSize converts a size in DIPs to the physical pixels the window manager
// expects, keeping the result inside the screen so a large scale factor does not
// push the window off the desktop.
func windowSize(dipWidth, dipHeight uint) (uint, uint) {
	scale := 1.0
	if dpi, _, _ := procGetDpiForSystem.Call(); dpi > 0 {
		scale = float64(dpi) / 96.0
	}
	w := uint(float64(dipWidth) * scale)
	h := uint(float64(dipHeight) * scale)

	screenW, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
	screenH, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
	if screenW > 0 {
		if max := uint(float64(screenW) * 0.92); w > max {
			w = max
		}
	}
	if screenH > 0 {
		if max := uint(float64(screenH) * 0.88); h > max {
			h = max
		}
	}
	return w, h
}

func newPanelApp(cfg *Config, srv *panelServer, svc *service) (*panelApp, error) {
	dataPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "MetaTubePanel", "WebView2")
	_ = os.MkdirAll(dataPath, 0o755)

	// The window size below is in physical pixels, so it has to be scaled by the
	// display's DPI. Without this the panel opened as a small window on a 4K
	// screen at 175%.
	width, height := windowSize(1380, 900)

	wv := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  "MetaTube 控制台",
			Width:  width,
			Height: height,
			Center: true,
		},
	})
	if wv == nil {
		return nil, errors.New("WebView2 初始化失败：请确认已安装 Microsoft Edge WebView2 运行时")
	}

	a := &panelApp{cfg: cfg, srv: srv, svc: svc, wv: wv, hwnd: windows.HWND(uintptr(wv.Window()))}
	currentApp = a

	a.orig = setWindowLongPtr(a.hwnd, gwlpWndProc, syscall.NewCallback(windowProc))
	// All three come from the official logo embedded in the executable's
	// resources; Windows picks the closest one for each context.
	a.iconSmall = createAppIcon(16)
	a.icon = createAppIcon(32)
	a.iconBig = createAppIcon(48)
	if a.icon == 0 {
		a.icon = a.iconBig
	}
	if a.icon == 0 {
		a.icon = a.iconSmall
	}
	if a.iconBig == 0 {
		a.iconBig = a.icon
	}
	if a.iconSmall == 0 {
		a.iconSmall = a.icon
	}
	if a.iconBig != 0 {
		procSendMessageW.Call(uintptr(a.hwnd), wmSetIcon, iconBig, uintptr(a.iconBig))
	}
	if a.iconSmall != 0 {
		procSendMessageW.Call(uintptr(a.hwnd), wmSetIcon, iconSmall, uintptr(a.iconSmall))
	}
	a.addTray()
	return a, nil
}

func setWindowLongPtr(hwnd windows.HWND, index uintptr, value uintptr) uintptr {
	r, _, _ := procSetWindowLongPtrW.Call(uintptr(hwnd), index, value)
	return r
}

func callWindowProc(proc, hwnd, msg, wp, lp uintptr) uintptr {
	r, _, _ := procCallWindowProcW.Call(proc, hwnd, msg, wp, lp)
	return r
}

func windowProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := currentApp
	if a == nil {
		return 0
	}
	switch msg {
	case wmSysCommand:
		// Minimize and the caption's X both fold the window into the tray.
		switch wp & 0xFFF0 {
		case scMinimize, scClose:
			a.hideToTray()
			return 0
		}
	case wmClose:
		a.hideToTray()
		return 0
	case wmSize:
		if wp == sizeMinimized {
			a.hideToTray()
			return 0
		}
	case trayCallback:
		switch uint32(lp & 0xFFFF) {
		case wmLButtonUp, wmLButtonDbl:
			a.showWindow()
			return 0
		case wmRButtonUp, wmContextMenu:
			a.showTrayMenu()
			return 0
		}
	case wmCommand:
		a.handleMenu(int(wp & 0xFFFF))
		return 0
	case wmAppQuit:
		procPostQuitMessage.Call(0)
		return 0
	}
	return callWindowProc(a.orig, hwnd, msg, wp, lp)
}

func (a *panelApp) hideToTray() {
	if a.exiting.Load() {
		return
	}
	procShowWindow.Call(uintptr(a.hwnd), swHide)
	a.inTray.Store(true)
}

func (a *panelApp) showWindow() {
	procShowWindow.Call(uintptr(a.hwnd), swRestore)
	procSetForeground.Call(uintptr(a.hwnd))
	a.inTray.Store(false)
}

// focusFromHTTP is called by /panel/focus on the HTTP goroutine; the window
// calls must run on the UI thread, so nudge the window proc instead.
func (a *panelApp) focusFromHTTP() {
	procPostMessageW.Call(uintptr(a.hwnd), trayCallback, 0, uintptr(wmLButtonUp))
}

func (a *panelApp) quit() {
	if !a.exiting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		a.svc.StopOnExit()
		a.removeTray()
		procPostMessageW.Call(uintptr(a.hwnd), wmAppQuit, 0, 0)
	}()
}

func (a *panelApp) handleMenu(id int) {
	switch id {
	case menuShow:
		a.showWindow()
	case menuStart:
		go func() {
			if err := a.svc.Start(); err != nil {
				a.balloon("启动失败", err.Error())
			}
		}()
	case menuStop:
		go func() {
			if err := a.svc.Stop(); err != nil {
				a.balloon("停止失败", err.Error())
			}
		}()
	case menuRestart:
		go func() {
			if err := a.svc.Restart(); err != nil {
				a.balloon("重启失败", err.Error())
			}
		}()
	case menuExit:
		a.quit()
	}
}

/* ---------- tray icon ---------- */

func fillUTF16(dst []uint16, s string) {
	src := windows.StringToUTF16(s)
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(dst)-1] = 0
	}
	copy(dst, src)
}

func (a *panelApp) trayTip() string {
	state := "服务未运行"
	if st, _ := a.svc.Status()["state"].(string); st == stateRunning || st == stateExternal {
		state = "服务已运行"
	}
	return "MetaTube 控制台 · " + state
}

func (a *panelApp) addTray() {
	a.nid = notifyIconData{
		CbSize:           uint32(unsafe.Sizeof(a.nid)),
		HWnd:             a.hwnd,
		UID:              trayUID,
		UFlags:           nifMessage | nifIcon | nifTip,
		UCallbackMessage: trayCallback,
		HIcon:            a.icon,
	}
	fillUTF16(a.nid.SzTip[:], a.trayTip())
	if r, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&a.nid))); r == 0 {
		log.Printf("添加托盘图标失败：%v", err)
	}
}

func (a *panelApp) removeTray() {
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&a.nid)))
}

func (a *panelApp) balloon(title, msg string) {
	nid := a.nid
	nid.UFlags = nifInfo
	nid.DwInfoFlags = 0x1 // NIIF_INFO
	fillUTF16(nid.SzInfoTitle[:], title)
	fillUTF16(nid.SzInfo[:], msg)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func appendMenuItem(menu uintptr, flags uint32, id int, text string) {
	p, _ := windows.UTF16PtrFromString(text)
	procAppendMenuW.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(p)))
}

func (a *panelApp) showTrayMenu() {
	fillUTF16(a.nid.SzTip[:], a.trayTip())
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&a.nid)))

	st := a.svc.Status()
	state, _ := st["state"].(string)
	running := state == stateRunning || state == stateExternal

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	appendMenuItem(menu, mfString, menuShow, "打开面板")
	appendMenuItem(menu, mfSeparator, 0, "")
	if running {
		appendMenuItem(menu, mfString|mfGrayed, menuStart, "启动服务")
		appendMenuItem(menu, mfString, menuStop, "停止服务")
	} else {
		appendMenuItem(menu, mfString, menuStart, "启动服务")
		appendMenuItem(menu, mfString|mfGrayed, menuStop, "停止服务")
	}
	appendMenuItem(menu, mfString, menuRestart, "重启服务")
	appendMenuItem(menu, mfSeparator, 0, "")
	appendMenuItem(menu, mfString, menuExit, "退出（关闭面板与服务）")

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// The window must be foreground or the menu will not close on click-away.
	procSetForeground.Call(uintptr(a.hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(menu,
		tpmRightButton|tpmBottomAlign, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(a.hwnd), 0)
	procPostMessageW.Call(uintptr(a.hwnd), wmNull, 0, 0)
	if cmd != 0 {
		a.handleMenu(int(cmd))
	}
}

/* ---------- icon ---------- */

// createAppIcon loads one size from the embedded official ICO and turns it
// into an HICON. The .syso resource linked into this binary supplies the
// Explorer file icon; this covers the window, taskbar and tray at runtime.
func createAppIcon(size int) windows.Handle {
	dib, err := iconres.DIB(size)
	if err != nil {
		log.Printf("加载图标(%d)失败：%v", size, err)
		return 0
	}
	h, _, _ := procCreateIconFromRes.Call(
		uintptr(unsafe.Pointer(&dib[0])), uintptr(len(dib)), 1,
		0x00030000 /* version 3 */, uintptr(size), uintptr(size), 0)
	return windows.Handle(h)
}

// destroyIcons releases the HICON handles once the window loop has ended.
func (a *panelApp) destroyIcons() {
	seen := make(map[windows.Handle]bool)
	for _, h := range []windows.Handle{a.iconSmall, a.icon, a.iconBig} {
		if h != 0 && !seen[h] {
			seen[h] = true
			procDestroyIcon.Call(uintptr(h))
		}
	}
}
