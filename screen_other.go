//go:build !windows

package mimickit

// realScreenSize returns a sane default on non-Windows platforms. mimic is a
// Windows-only core today; this stub only exists so the module compiles and
// unit-tests run on any OS.
func realScreenSize() (w, h int) {
	return 1920, 1080
}
