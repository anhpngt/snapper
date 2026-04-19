//go:build windows

package win

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/anhpngt/snapper/internal/action"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetWindowRect         = user32.NewProc("GetWindowRect")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procShowWindow            = user32.NewProc("ShowWindow")
	procIsZoomed              = user32.NewProc("IsZoomed")
	procDwmGetWindowAttribute = dwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	dwmwaExtendedFrameBounds = 9

	SW_MAXIMIZE = 3
	SW_RESTORE  = 9

	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010
)

type HWND uintptr

func GetForegroundWindow() HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return HWND(r)
}

func GetWindowRect(h HWND) (action.Rect, error) {
	var r action.Rect
	ret, _, err := procGetWindowRect.Call(uintptr(h), uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return r, fmt.Errorf("GetWindowRect: %w", err)
	}
	return r, nil
}

// GetExtendedFrameBounds returns the DWM-reported "visual" bounds of the
// window, which exclude the invisible resize border / drop shadow that
// GetWindowRect includes.
func GetExtendedFrameBounds(h HWND) (action.Rect, error) {
	var r action.Rect
	hr, _, _ := procDwmGetWindowAttribute.Call(
		uintptr(h),
		uintptr(dwmwaExtendedFrameBounds),
		uintptr(unsafe.Pointer(&r)),
		unsafe.Sizeof(r),
	)
	if hr != 0 {
		return r, fmt.Errorf("DwmGetWindowAttribute: HRESULT=0x%x", hr)
	}
	return r, nil
}

func IsZoomed(h HWND) bool {
	r, _, _ := procIsZoomed.Call(uintptr(h))
	return r != 0
}

func ShowWindow(h HWND, cmd int) {
	procShowWindow.Call(uintptr(h), uintptr(cmd))
}

func SetWindowPos(h HWND, x, y, cx, cy int32, flags uint32) error {
	ret, _, err := procSetWindowPos.Call(
		uintptr(h), 0,
		uintptr(x), uintptr(y), uintptr(cx), uintptr(cy),
		uintptr(flags),
	)
	if ret == 0 {
		return fmt.Errorf("SetWindowPos: %w", err)
	}
	return nil
}
