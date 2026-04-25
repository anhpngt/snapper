//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var procMessageBoxW = user32.NewProc("MessageBoxW")

const (
	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010
)

// MessageBox shows a native Windows message box. Use it for install/uninstall
// feedback where no console is available.
func MessageBox(title, text string, style uint32) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(style),
	)
}
