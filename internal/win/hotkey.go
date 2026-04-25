//go:build windows

package win

import (
	"fmt"
	"runtime"
	"unsafe"
)

var (
	procRegisterHotKey   = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procGetMessageW      = user32.NewProc("GetMessageW")
)

const (
	MOD_ALT      = 0x0001
	MOD_CONTROL  = 0x0002
	MOD_SHIFT    = 0x0004
	MOD_WIN      = 0x0008
	MOD_NOREPEAT = 0x4000

	VK_LEFT  = 0x25
	VK_UP    = 0x26
	VK_RIGHT = 0x27
	VK_DOWN  = 0x28

	wmHotkey = 0x0312
)

type HotKey struct {
	ID   int
	Mods uint32
	VK   uint32
}

type msg struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

// RunHotkeyLoop registers each hotkey, then blocks running a Win32 message
// loop on the calling OS thread. It sends the hotkey ID down `out` whenever
// a WM_HOTKEY arrives. It returns when GetMessage returns 0 (WM_QUIT) or on
// a fatal error. Must be called from a goroutine that has locked its OS
// thread - this function does that itself.
func RunHotkeyLoop(hotkeys []HotKey, out chan<- int) error {
	runtime.LockOSThread()

	for _, hk := range hotkeys {
		ret, _, err := procRegisterHotKey.Call(0, uintptr(hk.ID), uintptr(hk.Mods), uintptr(hk.VK))
		if ret == 0 {
			return fmt.Errorf("RegisterHotKey id=%d: %w", hk.ID, err)
		}
	}
	defer func() {
		for _, hk := range hotkeys {
			procUnregisterHotKey.Call(0, uintptr(hk.ID))
		}
	}()

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("GetMessage failed")
		}
		if ret == 0 {
			return nil
		}
		if m.Message == wmHotkey {
			out <- int(m.WParam)
		}
	}
}
