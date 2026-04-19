//go:build windows

package main

import (
	"log"
	"os"
	"slices"
	"time"

	"github.com/anhpngt/snapper/internal/action"
	"github.com/anhpngt/snapper/internal/install"
	"github.com/anhpngt/snapper/internal/win"
)

const (
	hotkeyLeft      = 1
	hotkeyRight     = 2
	hotkeyMoveLeft  = 3
	hotkeyMoveRight = 4
	hotkeyMaximize  = 5
)

var cycleSequence = []float64{0.5, 2.0 / 3.0, 1.0 / 3.0}

type binding struct {
	action       action.Action
	crossMonitor bool
	name         string
}

func main() {
	if err := install.Init(); err != nil {
		win.MessageBox("Snapper",
			"cannot resolve LocalAppData: "+err.Error(),
			win.MB_OK|win.MB_ICONERROR)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			runInstall()
			return
		case "uninstall":
			quiet := slices.Contains(os.Args[2:], "--quiet")
			purge := slices.Contains(os.Args[2:], "--purge")
			runUninstall(purge, quiet)
			return
		case "run":
			// explicit daemon mode
		default:
			win.MessageBox("Snapper", "Unknown command: "+os.Args[1]+
				"\n\nUsage:\n  snapper.exe               (install or run)\n  snapper.exe install\n  snapper.exe uninstall [--purge] [--quiet]",
				win.MB_OK|win.MB_ICONWARNING)
			os.Exit(2)
		}
	} else if !isInstalledLocation() {
		runInstall()
		return
	}

	runDaemon()
}

func isInstalledLocation() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return install.IsRunningInstalled(exe)
}

func runInstall() {
	if err := install.Install(); err != nil {
		win.MessageBox("Snapper install failed", err.Error(), win.MB_OK|win.MB_ICONERROR)
		os.Exit(1)
	}
	win.MessageBox("Snapper",
		"Installed to "+install.Dir()+
			".\n\nCtrl+Alt+Left/Right to snap. Ctrl+Shift+Alt+Left/Right to move across monitors."+
			"\n\nSnapper will start automatically at login. Uninstall via Settings > Apps.",
		win.MB_OK|win.MB_ICONINFORMATION)
}

func runUninstall(purge, quiet bool) {
	err := install.Uninstall(purge)
	if quiet {
		if err != nil {
			os.Exit(1)
		}
		return
	}
	if err != nil {
		win.MessageBox("Snapper uninstall failed", err.Error(), win.MB_OK|win.MB_ICONERROR)
		os.Exit(1)
	}
	win.MessageBox("Snapper", "Uninstalled.", win.MB_OK|win.MB_ICONINFORMATION)
}

func runDaemon() {
	if err := setupLogging(); err != nil {
		win.MessageBox("Snapper", "log setup: "+err.Error(), win.MB_OK|win.MB_ICONWARNING)
	}

	_, ok, err := install.AcquireSingleInstance()
	if err != nil {
		log.Printf("single-instance: %v", err)
		os.Exit(1)
	}
	if !ok {
		log.Println("another snapper instance is running; exiting")
		return
	}

	quitHandle, err := install.CreateQuitEvent()
	if err != nil {
		log.Printf("quit event: %v", err)
	} else {
		go func() {
			install.WaitForQuit(quitHandle)
			log.Println("quit event signaled; exiting")
			os.Exit(0)
		}()
	}

	if err := win.SetDpiAware(); err != nil {
		log.Printf("dpi: %v (continuing)", err)
	}

	cycler := action.NewCycler(cycleSequence, 10*time.Second, 4)

	events := make(chan int, 8)
	hotkeys := []win.HotKey{
		{ID: hotkeyLeft, Mods: win.MOD_CONTROL | win.MOD_ALT | win.MOD_NOREPEAT, VK: win.VK_LEFT},
		{ID: hotkeyRight, Mods: win.MOD_CONTROL | win.MOD_ALT | win.MOD_NOREPEAT, VK: win.VK_RIGHT},
		{ID: hotkeyMaximize, Mods: win.MOD_CONTROL | win.MOD_ALT | win.MOD_NOREPEAT, VK: win.VK_UP},
		{ID: hotkeyMoveLeft, Mods: win.MOD_CONTROL | win.MOD_ALT | win.MOD_SHIFT | win.MOD_NOREPEAT, VK: win.VK_LEFT},
		{ID: hotkeyMoveRight, Mods: win.MOD_CONTROL | win.MOD_ALT | win.MOD_SHIFT | win.MOD_NOREPEAT, VK: win.VK_RIGHT},
	}
	bindings := map[int]binding{
		hotkeyLeft:      {action: action.LeftHalf, name: "LeftHalf"},
		hotkeyRight:     {action: action.RightHalf, name: "RightHalf"},
		hotkeyMaximize:  {action: action.Maximize, name: "Maximize"},
		hotkeyMoveLeft:  {action: action.LeftHalf, crossMonitor: true, name: "LeftHalfOtherMonitor"},
		hotkeyMoveRight: {action: action.RightHalf, crossMonitor: true, name: "RightHalfOtherMonitor"},
	}

	go func() {
		defer close(events)
		if err := win.RunHotkeyLoop(hotkeys, events); err != nil {
			log.Printf("hotkey loop: %v", err)
			os.Exit(1)
		}
	}()

	log.Printf("snapper %s running (installed at %s)", install.Version, install.ExePath())

	for id := range events {
		b, ok := bindings[id]
		if !ok {
			continue
		}
		log.Printf("hotkey: %s", b.name)
		if err := snap(cycler, b.action, b.crossMonitor); err != nil {
			log.Printf("snap: %v", err)
		}
	}
}

// logRotateMaxBytes is the size threshold past which the current log gets
// renamed to .1 on startup and a fresh log begins. snapper writes a few bytes
// per hotkey press, so rotation realistically fires after months or years of
// use; one backup is plenty.
const logRotateMaxBytes = 1 * 1024 * 1024

func setupLogging() error {
	if err := os.MkdirAll(install.Dir(), 0o755); err != nil {
		return err
	}
	rotateLogIfLarge(install.LogPath(), logRotateMaxBytes)
	f, err := os.OpenFile(install.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	log.SetOutput(f)
	return nil
}

func rotateLogIfLarge(path string, maxBytes int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < maxBytes {
		return
	}
	backup := path + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(path, backup)
}

func snap(c *action.Cycler, a action.Action, crossMonitor bool) error {
	hwnd := win.GetForegroundWindow()
	if hwnd == 0 {
		return nil
	}

	if a == action.Maximize {
		win.ShowWindow(hwnd, win.SW_MAXIMIZE)
		return nil
	}

	var (
		work action.Rect
		err  error
	)
	if crossMonitor {
		var ok bool
		work, ok, err = win.AdjacentWorkArea(hwnd, a == action.RightHalf)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	} else {
		work, err = win.WorkArea(hwnd)
		if err != nil {
			return err
		}
	}

	if win.IsZoomed(hwnd) {
		win.ShowWindow(hwnd, win.SW_RESTORE)
	}

	winRect, err := win.GetWindowRect(hwnd)
	if err != nil {
		return err
	}
	dwmRect, err := win.GetExtendedFrameBounds(hwnd)
	if err != nil {
		dwmRect = winRect
	}

	// GetWindowRect includes the invisible drop-shadow margin on Win10/11;
	// DwmGetWindowAttribute(EXTENDED_FRAME_BOUNDS) returns the visible outline.
	// Expand our target outward by the per-side shadow so the visible rect
	// lands edge-to-edge.
	offLeft := dwmRect.Left - winRect.Left
	offTop := dwmRect.Top - winRect.Top
	offRight := winRect.Right - dwmRect.Right
	offBottom := winRect.Bottom - dwmRect.Bottom

	now := time.Now()
	frac := c.Next(uintptr(hwnd), a, dwmRect, now)
	target := action.Target(work, a, frac)

	x := target.Left - offLeft
	y := target.Top - offTop
	w := target.Width() + offLeft + offRight
	h := target.Height() + offTop + offBottom

	if err := win.SetWindowPos(hwnd, x, y, w, h, win.SWP_NOZORDER|win.SWP_NOACTIVATE); err != nil {
		return err
	}
	postDwm, errDwm := win.GetExtendedFrameBounds(hwnd)
	if errDwm != nil {
		postDwm = target
	}
	c.Applied(uintptr(hwnd), a, postDwm, now)
	return nil
}
