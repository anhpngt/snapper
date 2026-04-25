//go:build windows

package install

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

const (
	mutexName = `Local\Snapper.Instance`
	eventName = `Local\Snapper.Quit`
)

// AcquireSingleInstance creates the named mutex. Returns ok=false if another
// instance already holds it - caller should exit silently. On ok=true the
// returned handle must stay open for the life of the process so the mutex
// stays owned.
func AcquireSingleInstance() (handle windows.Handle, ok bool, err error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return 0, false, err
	}
	h, createErr := windows.CreateMutex(nil, false, name)
	if h == 0 {
		return 0, false, fmt.Errorf("CreateMutex: %w", createErr)
	}
	// CreateMutex sets GetLastError to ERROR_ALREADY_EXISTS when another
	// process already owns the named mutex, even though the returned handle
	// is valid. That handle is not ours, close it and report not-acquired.
	if errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return 0, false, nil
	}
	return h, true, nil
}

// CreateQuitEvent creates the auto-reset event the daemon waits on. Auto-reset
// is important: when the daemon wakes up from SignalQuit, the kernel clears
// the event atomically. A subsequent daemon opening the same named event sees
// it unsignaled - without this, a fast upgrade would have the new daemon
// inherit the leftover signal and exit immediately.
func CreateQuitEvent() (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(eventName)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateEvent(nil, 0, 0, name)
	if h == 0 {
		return 0, fmt.Errorf("CreateEvent: %w", err)
	}
	// Belt-and-suspenders: if the event already existed (unlikely but
	// possible if cleanup was partial), ensure it starts unsignaled.
	windows.ResetEvent(h)
	return h, nil
}

// WaitForQuit blocks until the quit event is signaled.
func WaitForQuit(h windows.Handle) {
	windows.WaitForSingleObject(h, windows.INFINITE)
}

// SignalQuit opens the quit event (polling briefly for it to exist, since a
// just-launched daemon may not have created it yet), sets it to tell the
// daemon to exit, then polls the instance mutex until it's released
// (indicating the old daemon has fully exited) or the timeout expires.
// Returns true if a daemon was signaled. If no event appears within the
// timeout, there's no running daemon to signal and this returns false.
func SignalQuit(timeout time.Duration) bool {
	evName, err := windows.UTF16PtrFromString(eventName)
	if err != nil {
		return false
	}
	deadline := time.Now().Add(timeout)

	var h windows.Handle
	for {
		h, err = windows.OpenEvent(windows.EVENT_MODIFY_STATE, false, evName)
		if err == nil && h != 0 {
			break
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(25 * time.Millisecond)
	}
	defer windows.CloseHandle(h)

	if err := windows.SetEvent(h); err != nil {
		return false
	}

	// Give the daemon at least 500ms to process the signal and release
	// its mutex, even if most of the timeout was consumed by the existence
	// poll above.
	remaining := max(time.Until(deadline), 500*time.Millisecond)
	waitForMutexRelease(remaining)
	return true
}

// waitForMutexRelease polls the single-instance mutex. When OpenMutex fails
// with ERROR_FILE_NOT_FOUND, the mutex has been destroyed, meaning every
// handle to it has been closed - the daemon is truly gone.
func waitForMutexRelease(timeout time.Duration) {
	mxName, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, mxName)
		if err != nil {
			// File-not-found (or any error opening) means the mutex is
			// gone; we don't distinguish sub-cases because the only
			// accessibly-open case is "still alive".
			return
		}
		windows.CloseHandle(h)
		time.Sleep(25 * time.Millisecond)
	}
}
