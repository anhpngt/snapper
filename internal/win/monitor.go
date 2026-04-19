//go:build windows

package win

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/anhpngt/snapper/internal/action"
)

var (
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")

	enumMu       sync.Mutex
	enumMonitors []monitor
	enumErr      error
	enumCallback = syscall.NewCallback(enumDisplayMonitorCallback)
)

const monitorDefaultToNearest = 2

type monitor struct {
	Handle  uintptr
	Monitor action.Rect
	Work    action.Rect
}

type monitorInfo struct {
	CbSize    uint32
	RcMonitor action.Rect
	RcWork    action.Rect
	DwFlags   uint32
}

// WorkArea returns the work area (full monitor rect minus taskbar) for the
// monitor that contains the given window.
func WorkArea(h HWND) (action.Rect, error) {
	mon, err := monitorFromWindow(h)
	if err != nil {
		return action.Rect{}, err
	}
	return mon.Work, nil
}

// AdjacentWorkArea returns the work area of the nearest monitor to the left or
// right of the window's current monitor. When no monitor exists in that
// direction, ok is false and err is nil.
func AdjacentWorkArea(h HWND, moveRight bool) (action.Rect, bool, error) {
	current, err := monitorFromWindow(h)
	if err != nil {
		return action.Rect{}, false, err
	}
	monitors, err := allMonitors()
	if err != nil {
		return action.Rect{}, false, err
	}
	next, ok := adjacentMonitor(monitors, current, moveRight)
	if !ok {
		return action.Rect{}, false, nil
	}
	return next.Work, true, nil
}

func monitorFromWindow(h HWND) (monitor, error) {
	mon, _, _ := procMonitorFromWindow.Call(uintptr(h), monitorDefaultToNearest)
	if mon == 0 {
		return monitor{}, fmt.Errorf("MonitorFromWindow returned null")
	}
	return monitorFromHandle(mon)
}

func monitorFromHandle(handle uintptr) (monitor, error) {
	mi := monitorInfo{}
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, err := procGetMonitorInfoW.Call(handle, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return monitor{}, fmt.Errorf("GetMonitorInfo: %w", err)
	}
	return monitor{
		Handle:  handle,
		Monitor: mi.RcMonitor,
		Work:    mi.RcWork,
	}, nil
}

func allMonitors() ([]monitor, error) {
	enumMu.Lock()
	defer enumMu.Unlock()

	enumMonitors = enumMonitors[:0]
	enumErr = nil

	ret, _, err := procEnumDisplayMonitors.Call(0, 0, enumCallback, 0)
	if ret == 0 {
		if enumErr != nil {
			return nil, enumErr
		}
		return nil, fmt.Errorf("EnumDisplayMonitors: %w", err)
	}

	out := make([]monitor, len(enumMonitors))
	copy(out, enumMonitors)
	return out, nil
}

func enumDisplayMonitorCallback(handle, _, _, _ uintptr) uintptr {
	mon, err := monitorFromHandle(handle)
	if err != nil {
		enumErr = err
		return 0
	}
	enumMonitors = append(enumMonitors, mon)
	return 1
}

func adjacentMonitor(monitors []monitor, current monitor, moveRight bool) (monitor, bool) {
	curX := rectMidX(current.Monitor)
	curY := rectMidY(current.Monitor)

	best := -1
	var bestDx int64
	var bestDy int64
	for i, mon := range monitors {
		if mon.Handle == current.Handle {
			continue
		}

		dx := rectMidX(mon.Monitor) - curX
		if moveRight {
			if dx <= 0 {
				continue
			}
		} else if dx >= 0 {
			continue
		}

		adx := abs64(dx)
		ady := abs64(rectMidY(mon.Monitor) - curY)
		if best == -1 || adx < bestDx || (adx == bestDx && ady < bestDy) {
			best = i
			bestDx = adx
			bestDy = ady
		}
	}
	if best == -1 {
		return monitor{}, false
	}
	return monitors[best], true
}

func rectMidX(r action.Rect) int64 {
	return int64(r.Left) + int64(r.Width())/2
}

func rectMidY(r action.Rect) int64 {
	return int64(r.Top) + int64(r.Height())/2
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
