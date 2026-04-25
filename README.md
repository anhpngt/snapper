# snapper

A small Windows window-snapping daemon. Like Rectangle on macOS: press a hotkey, the foreground window snaps. Press the same hotkey again to cycle through 1/2 -> 2/3 -> 1/3 of the screen.

Single binary, no dependencies, per-user install (no admin), runs on every login.

## Hotkeys

| Combo | Action |
|---|---|
| `Ctrl+Alt+Left` / `Ctrl+Alt+Right` | Snap the foreground window to the left or right half. Repeat to cycle 1/2 -> 2/3 -> 1/3. |
| `Ctrl+Alt+Up` | Maximize the foreground window. |
| `Ctrl+Shift+Alt+Left` / `Ctrl+Shift+Alt+Right` | Move the foreground window to the left/right adjacent monitor (same half-snap behavior on the new monitor). |

The cycle resets when you switch windows, switch directions, manually resize the window, or wait more than 10 seconds between presses.

## Install

Download `snapper.exe` and double-click it. You'll see a confirmation dialog. From that point on:

- The daemon runs in the background. No console, no tray icon.
- It auto-starts at every Windows login.
- Hotkeys are registered globally.
- Logs go to `%LOCALAPPDATA%\Snapper\snapper.log` (rotated at 1 MiB).

The Desktop copy can be deleted after install. The installed copy lives at `%LOCALAPPDATA%\Snapper\snapper.exe`.

To upgrade, just download a new `snapper.exe` and double-click it again. The running daemon is signaled to exit, the binary is replaced atomically, and the new daemon starts.

## Uninstall

Settings -> Apps -> Installed apps -> Snapper -> Uninstall.

Or from a terminal:

```
"%LOCALAPPDATA%\Snapper\snapper.exe" uninstall          # remove registry, keep files
"%LOCALAPPDATA%\Snapper\snapper.exe" uninstall --purge  # also delete %LOCALAPPDATA%\Snapper
```

## Build

Requires Go 1.26+. The binary is a Windows GUI subsystem executable (no console window).

```
GOOS=windows GOARCH=amd64 go build -ldflags="-H windowsgui -s -w" -o snapper.exe ./cmd/snapper
```

## Layout

- `internal/action/` - placement logic and the cycle state machine. Pure Go, no Win32, unit-tested.
- `internal/win/` - thin Win32 wrappers (hotkey, window, monitor, DPI, message box).
- `internal/install/` - install/uninstall, single-instance mutex, quit-event signaling, atomic copy, registry entries.
- `cmd/snapper/` - dispatch (install vs daemon), hotkey wiring, logging.

## Notes

- DPI: registers as Per-Monitor V2 DPI aware so coordinates from `GetWindowRect`, `GetMonitorInfo`, and `DwmGetWindowAttribute` are all in the same physical-pixel units. Required to make the DWM frame compensation math work; without it, snapped windows have an 8px shadow gap.
- Per-user only: everything goes in `HKCU` and `%LOCALAPPDATA%`. No admin required.
- Single instance: enforced via the named mutex `Local\Snapper.Instance`. Launching a second copy exits silently.
