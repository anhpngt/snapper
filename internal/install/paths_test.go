//go:build windows

package install

import "testing"

func TestSamePath_CaseAndSeparatorInsensitive(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{`C:\Users\foo\AppData\Local\Snapper\snapper.exe`,
			`c:\users\foo\appdata\local\snapper\snapper.exe`, true},
		{`C:\Users\foo\AppData\Local\Snapper\snapper.exe`,
			`C:\Users\foo\AppData\Local\Snapper\.\snapper.exe`, true},
		{`C:\Users\foo\AppData\Local\Snapper\snapper.exe`,
			`C:\Users\foo\AppData\Local\Snapper\sub\..\snapper.exe`, true},
		{`C:\Users\foo\AppData\Local\Snapper\snapper.exe`,
			`C:\Users\foo\Desktop\snapper.exe`, false},
		{`C:\Users\foo\AppData\Local\Snapper\snapper.exe`,
			`D:\Users\foo\AppData\Local\Snapper\snapper.exe`, false},
	}
	for _, tc := range cases {
		if got := samePath(tc.a, tc.b); got != tc.want {
			t.Errorf("samePath(%q, %q) = %v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestPathsUseBaseDir(t *testing.T) {
	baseOnce.Do(func() {}) // consume the once so our override is the answer
	baseDir = `C:\Users\test\AppData\Local\Snapper`
	baseErr = nil
	t.Cleanup(func() {
		baseDir = ""
	})

	if got, want := Dir(), `C:\Users\test\AppData\Local\Snapper`; got != want {
		t.Errorf("Dir() = %q want %q", got, want)
	}
	if got, want := ExePath(), `C:\Users\test\AppData\Local\Snapper\snapper.exe`; got != want {
		t.Errorf("ExePath() = %q want %q", got, want)
	}
	if got, want := LogPath(), `C:\Users\test\AppData\Local\Snapper\snapper.log`; got != want {
		t.Errorf("LogPath() = %q want %q", got, want)
	}
}

func TestIsRunningInstalled(t *testing.T) {
	baseOnce.Do(func() {})
	baseDir = `C:\Users\test\AppData\Local\Snapper`
	baseErr = nil
	t.Cleanup(func() {
		baseDir = ""
	})

	if !IsRunningInstalled(`C:\Users\test\AppData\Local\Snapper\snapper.exe`) {
		t.Error("expected installed location to match")
	}
	if IsRunningInstalled(`C:\Users\test\Desktop\snapper.exe`) {
		t.Error("expected desktop location to not match installed path")
	}
}
