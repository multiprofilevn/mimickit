package mimickit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/multiprofilevn/mimickit/internal/standalone"
)

// chromeProc is a running chrome process abstracted for testability: the real
// spawner wraps exec.Cmd, tests inject a fake with closures.
type chromeProc struct {
	pid  int32
	kill func() error
	wait func() error // blocks until the process exits
}

// chromeSpawner starts chrome.exe with the given args and extra env (merged over
// the current environment) and returns a handle to the process.
type chromeSpawner func(exe string, args, env []string) (*chromeProc, error)

// standaloneSelected reports whether the adapter-less launcher should run.
// Default (Standalone nil) is TRUE — mimickit v0.2.0 no longer depends on
// adapter.exe; only an explicit Standalone=false falls back to the legacy path.
func standaloneSelected(opts Options) bool {
	return opts.Standalone == nil || *opts.Standalone
}

// standaloneDeps are the injectable seams of the standalone launch flow.
type standaloneDeps struct {
	spawn             chromeSpawner
	ensureCore        func(coreDir string, opts CoreOpts) (string, error)
	patchLocal        func(coreDir string) error
	resolveExtensions func() []string
}

func defaultStandaloneDeps() standaloneDeps {
	return standaloneDeps{
		spawn:             realSpawnChrome,
		ensureCore:        EnsureCore,
		patchLocal:        patchLocalCore,
		resolveExtensions: resolveMimicExtensions,
	}
}

// resolveMimicExtensions lists the unpacked-extension dirs to load — every direct
// subdir of CustomExtensionsDir. In adapter mode adapter.exe scans that folder;
// standalone spawns chrome directly, so we pass the same dirs via --load-extension.
func resolveMimicExtensions() []string {
	base := CustomExtensionsDir()
	if base == "" {
		return nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(base, e.Name()))
		}
	}
	return dirs
}

// patchCore is standalone.PatchCore behind a variable so tests can count calls
// without supplying a real multi-hundred-megabyte DLL.
var patchCore = standalone.PatchCore

// patchLocalCore ensures a pre-installed core's chrome.dll is patched so the
// standalone launch passes mimic's initial-check gate WITHOUT adapter.exe. It is
// idempotent and guarded by the patch marker so the ~100-200MB DLL is only read
// when a (re)patch is actually needed. A missing chrome.dll is left for the spawn
// step to surface with a clear error.
//
// The patch rewrites chrome.dll in place, so concurrent launches on a cold core
// would otherwise interleave their writes and corrupt it. Two locks prevent
// that — one for goroutines, one for other processes sharing the core — and the
// marker is re-checked after taking each: whoever waited finds the work already
// done and returns without touching the file. Checking before any lock keeps the
// common case, an already-patched core, free of lock traffic.
func patchLocalCore(coreDir string) error {
	chromeDLL := filepath.Join(coreDir, "chrome.dll")
	if !fileExists(chromeDLL) {
		return nil
	}
	ver := strings.TrimPrefix(filepath.Base(coreDir), "mimic_")
	if ver == "" {
		ver = "local"
	}
	if !needsRepatch(coreDir, ver) {
		return nil
	}

	mu := corePatchMutex(coreDir)
	mu.Lock()
	defer mu.Unlock()
	if !needsRepatch(coreDir, ver) {
		return nil
	}

	release, err := acquirePatchLock(coreDir)
	if err != nil {
		return err
	}
	defer release()
	if !needsRepatch(coreDir, ver) {
		return nil
	}

	if err := patchCore(chromeDLL); err != nil {
		return err
	}
	return writePatchMarker(coreDir, ver)
}

// defaultPortWhitelist mirrors the adapter's standard open-port set; used when
// the fingerprint supplies none.
var defaultPortWhitelist = []int{21, 22, 80, 443, 3389, 8080}

// launchStandalone runs the adapter-less flow: ensure/patch the core, map the
// fingerprint, start the fp-server, spawn chrome with the plugin-handshake env,
// and return a Handle whose Stop tears down both chrome and the fp-server. The
// Handle contract is identical to the adapter path so consumers don't change.
func launchStandalone(opts Options) (*Handle, error) {
	return launchStandaloneWith(opts, defaultStandaloneDeps())
}

func launchStandaloneWith(opts Options, deps standaloneDeps) (*Handle, error) {
	coreDir := ResolveCoreDir(opts.AdapterDir)
	if opts.CoreDownload != nil {
		// Download path: EnsureCore fetches + patches the core itself.
		target := coreDir
		if !fileExists(filepath.Join(coreDir, "chrome.dll")) {
			target = filepath.Join(mlxDepsDir(), "mimic_"+opts.CoreDownload.Version)
		}
		if _, err := deps.ensureCore(target, *opts.CoreDownload); err != nil {
			return nil, fmt.Errorf("ensure core: %w", err)
		}
		coreDir = target
	} else if deps.patchLocal != nil {
		// Pre-installed core: patch chrome.dll in place so standalone passes the
		// gate without adapter.exe.
		if err := deps.patchLocal(coreDir); err != nil {
			return nil, fmt.Errorf("patch local core: %w", err)
		}
	}
	chromeExe := filepath.Join(coreDir, "chrome.exe")

	profileDir := filepath.Join(opts.ProfileDir, opts.Name)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", profileDir, err)
	}

	cfg := MapFingerprint(resolveFingerprint(opts), resolveFlags(opts), opts.Proxy)
	cfg.SessionID = profileIDFromName(opts.Name)
	cfg.SessionName = displayNameOf(opts)
	cfg.UserDataDir = profileDir
	cfg.ManagedFlags = opts.ManagedFlags
	if len(cfg.PortWhitelist) == 0 {
		cfg.PortWhitelist = defaultPortWhitelist
	}
	// User args come AFTER any managed proxy args MapFingerprint emitted, so they
	// still override everything.
	cfg.ChromeArgs = append(cfg.ChromeArgs, opts.ChromeArgs...)
	if deps.resolveExtensions != nil {
		cfg.LoadExtensions = deps.resolveExtensions()
	}

	clientPort, err := standalone.FreePort()
	if err != nil {
		return nil, fmt.Errorf("alloc client port: %w", err)
	}
	rdp, err := standalone.FreePort()
	if err != nil {
		return nil, fmt.Errorf("alloc debug port: %w", err)
	}
	cfg.ClientPort = clientPort
	cfg.RemoteDebuggingPort = rdp

	payload, err := standalone.BuildFingerprintPayload(cfg)
	if err != nil {
		return nil, fmt.Errorf("build fingerprint: %w", err)
	}
	fpsrv, err := standalone.NewFPServer(payload)
	if err != nil {
		return nil, fmt.Errorf("fp server: %w", err)
	}
	// Bind BEFORE spawning chrome so the first fingerprint fetch succeeds.
	if _, err := fpsrv.Start(clientPort); err != nil {
		return nil, fmt.Errorf("start fp server: %w", err)
	}

	proc, err := deps.spawn(chromeExe, standalone.BuildChromeArgs(cfg), standalone.PluginHandshakeEnv())
	if err != nil {
		_ = fpsrv.Close()
		return nil, fmt.Errorf("spawn chrome: %w", err)
	}

	var exited atomic.Bool
	if proc.wait != nil {
		go func() { _ = proc.wait(); exited.Store(true) }()
	}

	cdpAddress := fmt.Sprintf("http://127.0.0.1:%d", rdp)
	// Automation callers (SkipCDPDiscovery=false) connect to CDP immediately after
	// Launch returns; unlike the adapter path (which discovers the port only once
	// chrome has bound it), standalone knows the port up front, so wait until it is
	// actually answering /json. Best-effort: never fail the launch — chrome may
	// still be coming up and the caller's Connect can retry.
	if !opts.SkipCDPDiscovery {
		if err := waitCDPReady(cdpAddress, cdpDiscoveryTimeout); err != nil {
			fmt.Fprintf(os.Stderr, "[mimickit] standalone CDP not ready on %s within %s: %v\n", cdpAddress, cdpDiscoveryTimeout, err)
		}
	}

	h := &Handle{
		ProfileID:  cfg.SessionID,
		CDPPort:    rdp,
		CDPAddress: cdpAddress,
		chromePID:  proc.pid,
		activeFn:   func() bool { return !exited.Load() },
		stopStandalone: func() error {
			exited.Store(true)
			var kerr error
			if proc.kill != nil {
				kerr = proc.kill()
			}
			_ = fpsrv.Close()
			return kerr
		},
	}
	return h, nil
}

// waitCDPReady polls <addr>/json until chrome answers or the deadline passes,
// reusing the adapter path's probe. Returns the last error on timeout.
func waitCDPReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := probeCDP(addr); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(150 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return lastErr
}

// resolveFlags returns the caller-supplied masking flags or the verified green
// defaults when none are provided. Proxy presence still forces "custom" later
// inside MapFingerprint.
func resolveFlags(opts Options) ProfileMetaFlags {
	if len(opts.Flags) > 0 {
		var f ProfileMetaFlags
		if err := json.Unmarshal(opts.Flags, &f); err == nil {
			return f
		}
	}
	return defaultGreenFlags()
}

func displayNameOf(opts Options) string {
	if opts.DisplayName != "" {
		return opts.DisplayName
	}
	return opts.Name
}

// realSpawnChrome starts chrome.exe detached-but-tracked: stdout/stderr are
// forwarded, the plugin-handshake env is merged over the process environment,
// and kill/wait are wired to the OS process.
func realSpawnChrome(exe string, args, env []string) (*chromeProc, error) {
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &chromeProc{
		pid:  int32(cmd.Process.Pid),
		kill: func() error { return cmd.Process.Kill() },
		wait: func() error { return cmd.Wait() },
	}, nil
}
