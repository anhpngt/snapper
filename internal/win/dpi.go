//go:build windows

package win

import "fmt"

var procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is defined as the handle value -4.
// SetProcessDpiAwarenessContext takes it as an opaque HANDLE.
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// SetDpiAware marks the process as per-monitor DPI aware (V2). This must be
// called before any window is created. Required on HiDPI displays so that
// GetWindowRect, GetMonitorInfo, and DwmGetWindowAttribute all return
// coordinates in the same (physical) units.
func SetDpiAware() error {
	ret, _, err := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	if ret == 0 {
		return fmt.Errorf("SetProcessDpiAwarenessContext: %w", err)
	}
	return nil
}
