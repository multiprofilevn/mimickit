//go:build windows

package mimickit

import "syscall"

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	smCxScreen = 0 // SM_CXSCREEN
	smCyScreen = 1 // SM_CYSCREEN
)

// realScreenSize returns the primary display resolution via Win32 GetSystemMetrics,
// falling back to 1920x1080 on any error.
func realScreenSize() (w, h int) {
	cw, _, _ := getSystemMetrics.Call(smCxScreen)
	ch, _, _ := getSystemMetrics.Call(smCyScreen)
	if cw == 0 || ch == 0 {
		return 1920, 1080
	}
	return int(cw), int(ch)
}
