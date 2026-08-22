package mimickit

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// adapterExeName is the plugin binary an ADAPTER-mode core directory must contain.
const adapterExeName = "adapter.exe"

// chromeExeName and chromeDLLName are the two files a STANDALONE launch needs.
// Standalone spawns chrome directly and never loads adapter.exe, and a core
// shipped as a zip contains no adapter.exe at all — so "is a usable core here"
// cannot be answered by adapter.exe alone.
const (
	chromeExeName = "chrome.exe"
	chromeDLLName = "chrome.dll"
)

// mlxDepsDir is the Multilogin install's core location under the user's home:
// <home>/mlx/deps.
//
// An UNPATCHED core refuses to start unless the genuine adapter launched it,
// crashing on startup (exit 0xe06d7363, "browser did not launched in time") —
// which is what a copied or repackaged core does when run on its own. That is a
// check on the launcher, not on the path: standalone NOPs it (see
// internal/standalone.PatchCore), after which the core runs from anywhere. So a
// core outside this tree is fine for standalone, and this dir is simply where a
// real Multilogin install keeps one.
func mlxDepsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "mlx", "deps")
}

// multiprofileCoreDir is where the multiprofile-v2 apps download mimic cores:
// <home>/.multiprofile/resources/browser/mimic_browser/<version>.
//
// This is one application's layout, not a mimic convention — it lives here only
// as a fallback for a caller that did not pass Options.AdapterDir. Passing the
// dir explicitly is the supported route, and it takes priority over this.
func multiprofileCoreDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".multiprofile", "resources", "browser", "mimic_browser")
}

// ResolveCoreDir returns the directory holding the core. Resolution priority
// (first hit wins):
//  1. explicit dir passed by caller — scanned like any base, so either the
//     browser folder or the version dir inside it works; returned unchanged when
//     it holds no core, so the caller's own path shows up in the error
//  2. env MLX_ADAPTER_DIR / ADAPTER_DIR
//  3. the Multilogin install: <home>/mlx/deps/mimic_*
//  4. the app's download dir: <home>/.multiprofile/.../mimic_browser/<version>
//  5. exe-relative: <exeDir>/cores/mimic_*, <exeDir>
//  6. cwd-relative: <cwd>/cores/mimic_*, <cwd>
//  7. last resort: <home>/mlx/deps itself, which may be empty
//
// The CLIENT launcher owns this — the core version reported to the adapter is
// derived from the folder actually installed here, never hardcoded by the API.
func ResolveCoreDir(explicit string) string {
	if explicit != "" {
		if d := findAdapterUnder(explicit); d != "" {
			return d
		}
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

// coreSearchBases returns directories to scan for the core, in priority order.
// A real Multilogin install (<home>/mlx/deps) is checked FIRST because it serves
// both launch modes; the app's own download dir follows for machines with no
// Multilogin installed, then exe/cwd-relative "cores" dirs for portable dev.
func coreSearchBases() []string {
	var bases []string
	if deps := mlxDepsDir(); deps != "" {
		bases = append(bases, deps)
	}
	if mp := multiprofileCoreDir(); mp != "" {
		bases = append(bases, mp)
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

// findAdapterUnder returns base if it directly holds a core, otherwise the
// highest-versioned subdir (e.g. mimic_149.2) that holds one. "A core" means
// either mode's core — see hasCore — because resolution must locate a directory
// before anything knows which mode will launch from it.
func findAdapterUnder(base string) string {
	if hasCore(base) {
		return base
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() && hasCore(filepath.Join(base, e.Name())) {
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

// hasCore reports whether dir holds a core usable by EITHER launch mode. It is
// the predicate for finding a directory, never for asserting that a particular
// mode can launch from it — the two mode guards below do that.
func hasCore(dir string) bool {
	return hasAdapter(dir) || hasStandaloneCore(dir)
}

// hasAdapter reports whether dir holds an adapter-mode core.
func hasAdapter(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, adapterExeName))
	return err == nil
}

// hasStandaloneCore reports whether dir holds a standalone-mode core: the chrome
// binary to spawn plus the DLL whose startup gate gets patched. Both are
// required — chrome.exe without chrome.dll cannot launch, and the patch step
// would silently no-op on it.
func hasStandaloneCore(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, chromeExeName)); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, chromeDLLName))
	return err == nil
}

// CoreExists reports whether an ADAPTER-mode core is resolvable right now. It is
// the guard for the legacy adapter path (Standalone=false); a standalone-only
// core correctly reports false here. Standalone callers want
// StandaloneCoreExists instead.
func CoreExists() bool {
	return CoreExistsIn("")
}

// CoreExistsIn is CoreExists for a caller that already knows where the core
// should be — pass the same dir given to Options.AdapterDir so the guard and the
// launch agree on which directory they are talking about. Empty behaves like
// CoreExists.
func CoreExistsIn(dir string) bool {
	return hasAdapter(ResolveCoreDir(dir))
}

// StandaloneCoreExists reports whether a STANDALONE-mode core is resolvable
// right now — the guard for the default adapter-less path.
func StandaloneCoreExists() bool {
	return StandaloneCoreExistsIn("")
}

// StandaloneCoreExistsIn is StandaloneCoreExists scoped to a known directory.
// See CoreExistsIn.
func StandaloneCoreExistsIn(dir string) bool {
	return hasStandaloneCore(ResolveCoreDir(dir))
}

// CoreVersion returns the resolved core folder name (e.g. "mimic_149.2") or ""
// if none is present. It accepts either mode's core: standalone derives its
// patch marker from this name, so gating on adapter.exe would leave a
// chrome-only core unversioned.
func CoreVersion(explicit string) string {
	d := ResolveCoreDir(explicit)
	if !hasCore(d) {
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
