//go:build windows

package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	Version   = "0.1.0"
	Publisher = "anhpngt"

	runKeyPath       = `Software\Microsoft\Windows\CurrentVersion\Run`
	uninstallKeyPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall\Snapper`

	// DETACHED_PROCESS | CREATE_BREAKAWAY_FROM_JOB - belt-and-suspenders for
	// Explorer-launched installs that may put the parent into a job object.
	detachedLaunchFlags = 0x00000008 | 0x01000000
)

// Install copies the current executable into the install directory, launches
// the installed copy, and writes the autostart and uninstall registry entries.
// Works for both fresh installs and upgrades; any running daemon is signaled
// to exit and wait-polled to disappear first so its exe can be overwritten.
//
// Step order is chosen so that registry entries claiming "Snapper is
// installed" are only written after we have a daemon running from the final
// location. A failure at any step short-circuits, and because the exe copy is
// atomic (tmp + rename), a partial failure never leaves a truncated binary.
func Install() (err error) {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current exe: %w", err)
	}

	SignalQuit(3 * time.Second)

	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	target := ExePath()
	if !samePath(currentExe, target) {
		if err := copyWithRetry(currentExe, target, 10, 200*time.Millisecond); err != nil {
			return fmt.Errorf("copy exe: %w", err)
		}
	}

	// Compensating actions run in reverse order on any subsequent error so
	// the caller never sees "install failed" with half the state committed.
	// The exe copy itself is intentionally not rolled back - a re-run of
	// install will overwrite it atomically and is the expected recovery.
	var rollback []func()
	defer func() {
		if err == nil {
			return
		}
		for i := len(rollback) - 1; i >= 0; i-- {
			rollback[i]()
		}
	}()

	if err = launchDetached(target); err != nil {
		return fmt.Errorf("launch daemon: %w", err)
	}
	rollback = append(rollback, func() {
		SignalQuit(3 * time.Second)
	})

	if err = writeRunEntry(target); err != nil {
		return fmt.Errorf("write autostart entry: %w", err)
	}
	rollback = append(rollback, func() {
		_ = deleteRunEntry()
	})

	if err = writeUninstallEntry(target); err != nil {
		return fmt.Errorf("write uninstall entry: %w", err)
	}
	return nil
}

// Uninstall signals the running daemon to exit, removes the registry entries,
// and (if purge) schedules deletion of the install directory. Safe to call
// when nothing is installed.
//
// Errors from individual steps are accumulated rather than aborted on, because
// uninstall is inherently convergent - every subsequent invocation of the
// same steps just moves closer to "all removed". Reporting all failures at
// once lets the caller see the full picture; re-running uninstall is safe.
func Uninstall(purge bool) error {
	SignalQuit(3 * time.Second)

	var errs []error
	if err := deleteRunEntry(); err != nil {
		errs = append(errs, fmt.Errorf("remove autostart entry: %w", err))
	}
	if err := deleteUninstallEntry(); err != nil {
		errs = append(errs, fmt.Errorf("remove uninstall entry: %w", err))
	}
	if purge {
		if err := schedulePurge(Dir()); err != nil {
			errs = append(errs, fmt.Errorf("schedule purge: %w", err))
		}
	}
	return errors.Join(errs...)
}

func writeRunEntry(exePath string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, `"`+exePath+`"`)
}

func deleteRunEntry() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer k.Close()
	err = k.DeleteValue(AppName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func writeUninstallEntry(exePath string) (err error) {
	k, openedExisting, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath, registry.WRITE)
	if err != nil {
		return err
	}
	// On a partial write (any SetValue below fails), delete the key only if
	// we just created it - never wipe out a pre-existing Uninstall entry from
	// a prior successful install, otherwise an upgrade that fails midway
	// leaves the user in worse shape than before.
	defer func() {
		k.Close()
		if err != nil && !openedExisting {
			_ = registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath)
		}
	}()

	strEntries := map[string]string{
		"DisplayName":          AppName,
		"DisplayVersion":       Version,
		"Publisher":            Publisher,
		"InstallLocation":      Dir(),
		"DisplayIcon":          exePath,
		"UninstallString":      fmt.Sprintf(`"%s" uninstall`, exePath),
		"QuietUninstallString": fmt.Sprintf(`"%s" uninstall --quiet`, exePath),
	}
	for name, value := range strEntries {
		if err = k.SetStringValue(name, value); err != nil {
			return err
		}
	}
	for name, value := range map[string]uint32{"NoModify": 1, "NoRepair": 1} {
		if err = k.SetDWordValue(name, value); err != nil {
			return err
		}
	}
	return nil
}

func deleteUninstallEntry() error {
	err := registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

func copyWithRetry(src, dst string, attempts int, pause time.Duration) error {
	var lastErr error
	for range attempts {
		if err := copyFileAtomic(src, dst); err != nil {
			lastErr = err
			time.Sleep(pause)
			continue
		}
		return nil
	}
	return lastErr
}

// copyFileAtomic copies src to a sibling temp file of dst and only renames it
// into place after the full copy succeeds. os.Rename on Windows is backed by
// MoveFileEx(MOVEFILE_REPLACE_EXISTING), which is atomic from the filesystem's
// perspective. A mid-copy failure leaves the existing dst untouched (or still
// absent) and the temp file gets cleaned up - no truncated installed binary.
func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".new"
	// Clean any leftover from a previous failed attempt.
	_ = os.Remove(tmp)

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// launchDetached spawns the installed exe so it survives this process exit
// and has no inherited console.
func launchDetached(path string) error {
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: detachedLaunchFlags,
	}
	return cmd.Start()
}

// schedulePurge spawns a detached cmd.exe that waits a couple of seconds and
// then removes the install directory. Used because the uninstaller itself is
// typically running from inside that directory and can't delete itself.
func schedulePurge(dir string) error {
	cmd := exec.Command("cmd.exe", "/c",
		`ping -n 3 127.0.0.1 >nul && rmdir /s /q "`+dir+`"`)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: detachedLaunchFlags,
	}
	return cmd.Start()
}

