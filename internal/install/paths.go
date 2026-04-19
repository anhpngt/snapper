//go:build windows

package install

import (
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/windows"
)

const (
	AppName = "Snapper"
	ExeName = "snapper.exe"
	LogName = "snapper.log"
)

var (
	baseOnce sync.Once
	baseDir  string
	baseErr  error
)

// resolveBase asks Windows for the canonical %LOCALAPPDATA% path via
// SHGetKnownFolderPath. We intentionally do not fall back to os.Getenv -
// relying on the env var lets a corrupt/overridden environment silently
// relocate the install, which defeats the "am I the installed copy?" check
// in main.
func resolveBase() (string, error) {
	baseOnce.Do(func() {
		p, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
		if err != nil {
			baseErr = err
			return
		}
		baseDir = filepath.Join(p, AppName)
	})
	return baseDir, baseErr
}

// Init resolves and caches the install directory. Main should call this
// early and surface the error; the Dir/ExePath/LogPath accessors assume it
// succeeded.
func Init() error {
	_, err := resolveBase()
	return err
}

// Dir is the per-user install directory (%LOCALAPPDATA%\Snapper). Returns
// empty string if Init was never called or failed.
func Dir() string {
	return baseDir
}

// ExePath is the canonical path of the installed executable.
func ExePath() string {
	return filepath.Join(Dir(), ExeName)
}

// LogPath is the canonical path of the daemon log file.
func LogPath() string {
	return filepath.Join(Dir(), LogName)
}

// IsRunningInstalled reports whether the given exe path matches the install
// location, in which case the process should run as the daemon rather than
// treating itself as an installer.
func IsRunningInstalled(currentExe string) bool {
	return samePath(currentExe, ExePath())
}

func samePath(a, b string) bool {
	a = strings.ToLower(filepath.Clean(a))
	b = strings.ToLower(filepath.Clean(b))
	return a == b
}
