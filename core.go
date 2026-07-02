package mimickit

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// DefaultAdapterDir is the dev-machine fallback where adapter.exe + patched
// chrome live when no explicit dir / env / portable core is found.
const DefaultAdapterDir = `C:\Users\zorovhs\mlx\deps\mimic_149.2`

// adapterExeName is the plugin binary the core directory must contain.
const adapterExeName = "adapter.exe"

// ResolveCoreDir returns the directory that contains adapter.exe. Resolution
// priority (first hit wins):
//  1. explicit dir passed by caller
//  2. env MLX_ADAPTER_DIR / ADAPTER_DIR
//  3. exe-relative: <exeDir>/cores/mimic_*, <exeDir>
//  4. cwd-relative: <cwd>/cores/mimic_*, <cwd>
//  5. DefaultAdapterDir (dev fallback)
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
	return DefaultAdapterDir
}

// coreSearchBases returns directories to scan for the core, exe-relative first.
func coreSearchBases() []string {
	var bases []string
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
