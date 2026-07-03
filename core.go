package mimickit

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// adapterExeName is the plugin binary the core directory must contain.
const adapterExeName = "adapter.exe"

// mlxDepsDir is the Multilogin install's core location under the user's home:
// <home>/mlx/deps. The patched Chromium core has an ANTI-TAMPER check that ties
// it to this install tree — running the core copied to any other path (e.g. a
// repackaged R2 download under the app's own resources dir) makes chrome crash
// on startup (exit 0xe06d7363, "browser did not launched in time"). So the core
// MUST be resolved from (or extracted into) <home>/mlx/deps/mimic_*.
func mlxDepsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "mlx", "deps")
}

// ResolveCoreDir returns the directory that contains adapter.exe. Resolution
// priority (first hit wins):
//  1. explicit dir passed by caller
//  2. env MLX_ADAPTER_DIR / ADAPTER_DIR
//  3. the Multilogin install: <home>/mlx/deps/mimic_* (anti-tamper-safe)
//  4. exe-relative: <exeDir>/cores/mimic_*, <exeDir>
//  5. cwd-relative: <cwd>/cores/mimic_*, <cwd>
//
// The CLIENT launcher owns this — the core version reported to the adapter is
// derived from the folder actually installed here, never hardcoded by the API.
func ResolveCoreDir(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("MLX_ADAPTER_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("ADAPTER_DIR"); v != "" {
		return v
	}
	for _, base := range coreSearchBases() {
		if d := findAdapterUnder(base); d != "" {
			return d
		}
	}
	// Last resort: the standard Multilogin deps dir itself (may be empty).
	if deps := mlxDepsDir(); deps != "" {
		return deps
	}
	return ""
}

// coreSearchBases returns directories to scan for the core. The Multilogin
// install (<home>/mlx/deps) is checked FIRST because the core only runs from
// there (anti-tamper); exe/cwd-relative "cores" dirs follow for portable dev.
func coreSearchBases() []string {
	var bases []string
	if deps := mlxDepsDir(); deps != "" {
		bases = append(bases, deps)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		bases = append(bases, filepath.Join(d, "cores"), d)
	}
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, filepath.Join(wd, "cores"), wd)
	}
	return bases
}

// findAdapterUnder returns base if it directly holds adapter.exe, otherwise the
// highest-versioned subdir (e.g. mimic_149.2) that holds adapter.exe.
func findAdapterUnder(base string) string {
	if hasAdapter(base) {
		return base
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() && hasAdapter(filepath.Join(base, e.Name())) {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 0 {
		return ""
	}
	// Reverse-sort by name so "mimic_149.2" > "mimic_148.0" picks newest.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return filepath.Join(base, matches[0])
}

func hasAdapter(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, adapterExeName))
	return err == nil
}

// CoreExists reports whether a usable core is resolvable right now.
func CoreExists() bool {
	return hasAdapter(ResolveCoreDir(""))
}

// CoreVersion returns the resolved core folder name (e.g. "mimic_149.2") or ""
// if none is present.
func CoreVersion(explicit string) string {
	d := ResolveCoreDir(explicit)
	if !hasAdapter(d) {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(d), string(filepath.Separator))
}

// coreVersions parses the resolved core folder ("mimic_149.2") into the values
// the adapter payload expects: the integer major used for core_version and the
// string browser_version. Falls back to (149, "149") if the folder name does
// not encode a version — 149 is the only pixelscan-verified green build today.
func coreVersions(explicit string) (major int, browserVersion string) {
	name := CoreVersion(explicit)
	suffix := strings.TrimPrefix(name, "mimic_")
	majorStr := suffix
	if i := strings.IndexByte(suffix, '.'); i >= 0 {
		majorStr = suffix[:i]
	}
	if n, err := strconv.Atoi(majorStr); err == nil && n > 0 {
		return n, majorStr
	}
	return 149, "149"
}
