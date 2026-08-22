package mimickit

import (
	"os"
	"path/filepath"
	"testing"
)

// newCoreDir creates <parent>/<name> and touches each named file inside it,
// returning the directory. Contents do not matter — resolution only stats.
func newCoreDir(t *testing.T, parent, name string, files ...string) string {
	t.Helper()
	dir := parent
	if name != "" {
		dir = filepath.Join(parent, name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s in %s: %v", f, dir, err)
		}
	}
	return dir
}

// isolateEnv points the home dir and cwd at a fresh temp dir and clears the
// adapter env overrides, so a core installed on the developer's real machine
// (or an MLX_ADAPTER_DIR set to work around the bug these tests cover) cannot
// leak into a test. Returns the temp dir standing in for $HOME and cwd.
//
// Both HOME and USERPROFILE are set: os.UserHomeDir reads USERPROFILE on
// Windows and HOME elsewhere, and these tests must pass on both.
func isolateEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("MLX_ADAPTER_DIR", "")
	t.Setenv("ADAPTER_DIR", "")
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Chdir(tmp)
	return tmp
}

// mlxDeps returns the <home>/mlx/deps path for an isolated home.
func mlxDeps(home string) string {
	return filepath.Join(home, "mlx", "deps")
}

func TestResolveCoreDir_ExplicitWins(t *testing.T) {
	home := isolateEnv(t)
	newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)
	envDir := newCoreDir(t, home, "env-core", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", envDir)

	explicit := filepath.Join(home, "explicit-core")
	if got := ResolveCoreDir(explicit); got != explicit {
		t.Errorf("ResolveCoreDir(explicit) = %q, want %q", got, explicit)
	}
}

func TestResolveCoreDir_MlxAdapterDirBeatsAdapterDir(t *testing.T) {
	home := isolateEnv(t)
	mlx := newCoreDir(t, home, "mlx-env", adapterExeName)
	other := newCoreDir(t, home, "adapter-env", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", mlx)
	t.Setenv("ADAPTER_DIR", other)

	if got := ResolveCoreDir(""); got != mlx {
		t.Errorf("ResolveCoreDir() = %q, want MLX_ADAPTER_DIR %q", got, mlx)
	}
}

func TestResolveCoreDir_EnvBeatsMlxDeps(t *testing.T) {
	home := isolateEnv(t)
	newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)
	envDir := newCoreDir(t, home, "env-core", adapterExeName)
	t.Setenv("ADAPTER_DIR", envDir)

	if got := ResolveCoreDir(""); got != envDir {
		t.Errorf("ResolveCoreDir() = %q, want env %q", got, envDir)
	}
}

func TestResolveCoreDir_MlxDepsVersionSubdir(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want %q", got, want)
	}
}

func TestResolveCoreDir_PicksHighestVersionDir(t *testing.T) {
	home := isolateEnv(t)
	newCoreDir(t, mlxDeps(home), "mimic_148.0", adapterExeName)
	want := newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want newest %q", got, want)
	}
}

func TestResolveCoreDir_FallsBackToDepsDirWhenEmpty(t *testing.T) {
	home := isolateEnv(t)

	if got := ResolveCoreDir(""); got != mlxDeps(home) {
		t.Errorf("ResolveCoreDir() = %q, want deps fallback %q", got, mlxDeps(home))
	}
}

func TestFindAdapterUnder_DirectHit(t *testing.T) {
	base := newCoreDir(t, t.TempDir(), "core", adapterExeName)

	if got := findAdapterUnder(base); got != base {
		t.Errorf("findAdapterUnder(base) = %q, want base %q", got, base)
	}
}

func TestFindAdapterUnder_NoMatch(t *testing.T) {
	base := newCoreDir(t, t.TempDir(), "empty")

	if got := findAdapterUnder(base); got != "" {
		t.Errorf("findAdapterUnder(empty) = %q, want empty string", got)
	}
}

func TestHasAdapter_EmptyDir(t *testing.T) {
	if hasAdapter("") {
		t.Error("hasAdapter(\"\") = true, want false")
	}
}

func TestCoreExists_TrueWhenAdapterPresent(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "core", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if !CoreExists() {
		t.Error("CoreExists() = false, want true when adapter.exe is present")
	}
}

// TestCoreExists_FalseWhenOnlyChrome pins the CURRENT meaning of CoreExists:
// it asks whether an ADAPTER core is resolvable, so a standalone-only core
// (chrome.exe + chrome.dll, no adapter.exe) does not satisfy it. Widening the
// resolution predicate must not change this — the adapter fallback path
// (MIMIC_STANDALONE=false) depends on it.
func TestCoreExists_FalseWhenOnlyChrome(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "core", "chrome.exe", "chrome.dll")
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if CoreExists() {
		t.Error("CoreExists() = true for a chrome-only core, want false")
	}
}

func TestCoreVersion_ReturnsFolderName(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "mimic_149.2", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if got := CoreVersion(""); got != "mimic_149.2" {
		t.Errorf("CoreVersion() = %q, want %q", got, "mimic_149.2")
	}
}

func TestCoreVersions_ParsesMimicPrefix(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "mimic_149.2", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	major, browser := coreVersions("")
	if major != 149 || browser != "149" {
		t.Errorf("coreVersions() = (%d, %q), want (149, \"149\")", major, browser)
	}
}

func TestCoreVersions_ParsesPlainVersion(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "149.0.7827.23", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	major, browser := coreVersions("")
	if major != 149 || browser != "149" {
		t.Errorf("coreVersions() = (%d, %q), want (149, \"149\")", major, browser)
	}
}

func TestCoreVersions_FallbackOnGarbage(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "not-a-version", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	major, browser := coreVersions("")
	if major != 149 || browser != "149" {
		t.Errorf("coreVersions() = (%d, %q), want the (149, \"149\") fallback", major, browser)
	}
}

// --- standalone core predicate -------------------------------------------------
//
// Standalone launches spawn chrome.exe directly and never touch adapter.exe, and
// a core downloaded as a zip ships without adapter.exe at all. Resolution must
// therefore find a chrome-only core, while the adapter-mode guard must keep
// treating one as absent.

// TestResolveCoreDir_FindsStandaloneOnlyDir is the defect these changes exist to
// fix: a perfectly usable standalone core was invisible to resolution.
func TestResolveCoreDir_FindsStandaloneOnlyDir(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, mlxDeps(home), "mimic_149.2", "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want chrome-only core %q", got, want)
	}
}

func TestResolveCoreDir_FindsAdapterOnlyDir(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want adapter-only core %q", got, want)
	}
}

func TestStandaloneCoreExists_TrueOnChromeCore(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "core", "chrome.exe", "chrome.dll")
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if !StandaloneCoreExists() {
		t.Error("StandaloneCoreExists() = false, want true for chrome.exe + chrome.dll")
	}
}

func TestStandaloneCoreExists_FalseWhenDLLMissing(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "core", "chrome.exe")
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if StandaloneCoreExists() {
		t.Error("StandaloneCoreExists() = true without chrome.dll, want false")
	}
}

func TestStandaloneCoreExists_FalseOnAdapterOnly(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "core", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if StandaloneCoreExists() {
		t.Error("StandaloneCoreExists() = true for an adapter-only core, want false")
	}
}

// TestCoreExists_UnchangedOnStandaloneOnlyDir guards the adapter fallback path:
// resolution now finds the chrome-only core, but CoreExists must still report it
// as no adapter core, or MIMIC_STANDALONE=false would launch a missing binary.
func TestCoreExists_UnchangedOnStandaloneOnlyDir(t *testing.T) {
	home := isolateEnv(t)
	newCoreDir(t, mlxDeps(home), "mimic_149.2", "chrome.exe", "chrome.dll")

	if CoreExists() {
		t.Error("CoreExists() = true for a chrome-only core, want false")
	}
}

func TestCoreVersion_WorksOnStandaloneOnlyDir(t *testing.T) {
	home := isolateEnv(t)
	dir := newCoreDir(t, home, "mimic_149.2", "chrome.exe", "chrome.dll")
	t.Setenv("MLX_ADAPTER_DIR", dir)

	if got := CoreVersion(""); got != "mimic_149.2" {
		t.Errorf("CoreVersion() = %q, want %q — the patch marker needs it", got, "mimic_149.2")
	}
}

func TestFindAdapterUnder_PrefersHighestAmongMixed(t *testing.T) {
	base := t.TempDir()
	newCoreDir(t, base, "mimic_148.0", adapterExeName)
	want := newCoreDir(t, base, "mimic_149.2", "chrome.exe", "chrome.dll")

	if got := findAdapterUnder(base); got != want {
		t.Errorf("findAdapterUnder() = %q, want newest %q", got, want)
	}
}

// --- explicit dir + the app's own resource dir ---------------------------------

// multiprofileBrowserDir mirrors the layout the multiprofile apps download into.
func multiprofileBrowserDir(home string) string {
	return filepath.Join(home, ".multiprofile", "resources", "browser", "mimic_browser")
}

// TestResolveCoreDir_ExplicitParentDirResolvesVersionSubdir covers the caller
// that knows the browser folder but not which version is installed in it.
func TestResolveCoreDir_ExplicitParentDirResolvesVersionSubdir(t *testing.T) {
	home := isolateEnv(t)
	parent := multiprofileBrowserDir(home)
	want := newCoreDir(t, parent, "149.0.7827.23", "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(parent); got != want {
		t.Errorf("ResolveCoreDir(parent) = %q, want version subdir %q", got, want)
	}
}

func TestResolveCoreDir_ExplicitVersionDirReturnsItself(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, multiprofileBrowserDir(home), "149.0.7827.23", "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(want); got != want {
		t.Errorf("ResolveCoreDir(versionDir) = %q, want itself %q", got, want)
	}
}

// TestResolveCoreDir_ExplicitEmptyDirReturnsAsIs keeps the caller's path in the
// eventual error message instead of silently substituting some other core.
func TestResolveCoreDir_ExplicitEmptyDirReturnsAsIs(t *testing.T) {
	home := isolateEnv(t)
	newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)
	explicit := newCoreDir(t, home, "empty-explicit")

	if got := ResolveCoreDir(explicit); got != explicit {
		t.Errorf("ResolveCoreDir(emptyDir) = %q, want it back unchanged %q", got, explicit)
	}
}

func TestResolveCoreDir_ExplicitStillBeatsEnv(t *testing.T) {
	home := isolateEnv(t)
	envDir := newCoreDir(t, home, "env-core", adapterExeName)
	t.Setenv("MLX_ADAPTER_DIR", envDir)
	want := newCoreDir(t, home, "explicit-core", "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(want); got != want {
		t.Errorf("ResolveCoreDir(explicit) = %q, want explicit %q to beat env", got, want)
	}
}

func TestCoreSearchBases_IncludesMultiprofileDir(t *testing.T) {
	home := isolateEnv(t)
	want := multiprofileBrowserDir(home)

	for _, b := range coreSearchBases() {
		if b == want {
			return
		}
	}
	t.Errorf("coreSearchBases() = %v, missing %q", coreSearchBases(), want)
}

func TestCoreSearchBases_MlxDepsBeforeMultiprofile(t *testing.T) {
	home := isolateEnv(t)
	bases := coreSearchBases()

	deps, multi := -1, -1
	for i, b := range bases {
		switch b {
		case mlxDeps(home):
			deps = i
		case multiprofileBrowserDir(home):
			multi = i
		}
	}
	if deps < 0 || multi < 0 {
		t.Fatalf("coreSearchBases() = %v, want both mlx/deps and the multiprofile dir", bases)
	}
	if deps > multi {
		t.Errorf("mlx/deps at %d, multiprofile at %d — a real Multilogin install must win", deps, multi)
	}
}

// TestResolveCoreDir_FindsMultiprofileCore reproduces the original bug report:
// the core was downloaded and complete, and resolution never looked at it.
func TestResolveCoreDir_FindsMultiprofileCore(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, multiprofileBrowserDir(home), "149.0.7827.23",
		adapterExeName, "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want the downloaded core %q", got, want)
	}
}

func TestResolveCoreDir_MlxDepsWinsOverMultiprofile(t *testing.T) {
	home := isolateEnv(t)
	want := newCoreDir(t, mlxDeps(home), "mimic_149.2", adapterExeName)
	newCoreDir(t, multiprofileBrowserDir(home), "149.0.7827.23", "chrome.exe", "chrome.dll")

	if got := ResolveCoreDir(""); got != want {
		t.Errorf("ResolveCoreDir() = %q, want the Multilogin install %q", got, want)
	}
}
