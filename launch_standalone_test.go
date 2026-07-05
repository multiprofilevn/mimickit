package mimickit

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeChrome captures the spawn call and lets the test drive process lifecycle.
type fakeChrome struct {
	exe    string
	args   []string
	env    []string
	killed bool
	exit   chan struct{}
}

func newFakeChrome() *fakeChrome { return &fakeChrome{exit: make(chan struct{})} }

func (f *fakeChrome) spawn(exe string, args, env []string) (*chromeProc, error) {
	f.exe, f.args, f.env = exe, args, env
	return &chromeProc{
		pid:  4242,
		kill: func() error { f.killed = true; return nil },
		wait: func() error { <-f.exit; return nil },
	}, nil
}

func fakeDeps(f *fakeChrome) standaloneDeps {
	return standaloneDeps{
		spawn:             f.spawn,
		ensureCore:        func(string, CoreOpts) (string, error) { return "149.2", nil },
		patchLocal:        func(string) error { return nil }, // no-op: don't touch a real local core in unit tests
		resolveExtensions: func() []string { return nil },    // no extensions in unit tests
	}
}

func argValue(args []string, prefix string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return strings.TrimPrefix(a, prefix), true
		}
	}
	return "", false
}

func TestStandaloneSelected_DefaultAndExplicit(t *testing.T) {
	if !standaloneSelected(Options{}) {
		t.Error("default (Standalone nil) should select standalone")
	}
	tr := true
	if !standaloneSelected(Options{Standalone: &tr}) {
		t.Error("Standalone=true should select standalone")
	}
	fa := false
	if standaloneSelected(Options{Standalone: &fa}) {
		t.Error("Standalone=false should select the adapter path")
	}
}

func TestLaunch_AdapterFallbackWhenFalse(t *testing.T) {
	// Explicit Standalone=false => legacy adapter path. With an empty AdapterDir
	// that path fails locating adapter.exe; the error proves it did NOT go
	// standalone.
	fa := false
	_, err := Launch(Options{Name: "x", AdapterDir: t.TempDir(), Standalone: &fa})
	if err == nil {
		t.Fatal("expected adapter-path error, got nil")
	}
	if !strings.Contains(err.Error(), "adapter") {
		t.Errorf("error = %v, want adapter-path error", err)
	}
}

func TestLaunchStandalone_BuildsConfig(t *testing.T) {
	fp, _ := json.Marshal(FingerprintData{
		Navigator: &FPNavigator{UserAgent: "UA-XYZ", Platform: "Win32", HardwareConcurrency: 8},
		Screen:    &FPScreen{Width: 1920, Height: 1080},
		Timezone:  &FPTimezone{Zone: "America/New_York"},
	})
	f := newFakeChrome()
	opts := Options{
		Name:             "prof-1",
		ProfileDir:       t.TempDir(),
		Fingerprint:      fp,
		Standalone:       boolTruePtr(),
		ChromeArgs:       []string{"--user-flag=1"},
		SkipCDPDiscovery: true, // don't block on a real CDP port in unit tests
	}
	h, err := launchStandaloneWith(opts, fakeDeps(f))
	if err != nil {
		t.Fatalf("launchStandaloneWith: %v", err)
	}
	defer h.Stop()

	if ua, ok := argValue(f.args, "--user-agent="); !ok || ua != "UA-XYZ" {
		t.Errorf("--user-agent = %q (ok=%v), want UA-XYZ", ua, ok)
	}
	if f.args[len(f.args)-1] != "--user-flag=1" {
		t.Errorf("last arg = %q, want --user-flag=1 (user args must be last)", f.args[len(f.args)-1])
	}
	cp, ok := argValue(f.args, "--client-port=")
	if !ok {
		t.Fatal("--client-port missing")
	}
	// client-port in args must match the CDP-independent fp-server port; the port
	// whitelist must include it as its first element.
	if pw, ok := argValue(f.args, "--port_whitelist="); !ok || !strings.HasPrefix(pw, "["+cp+",") {
		t.Errorf("port_whitelist = %q, want to start with the client port %s", pw, cp)
	}
	if h.CDPPort == 0 || h.CDPAddress == "" {
		t.Error("standalone Handle missing CDP endpoint")
	}
	if h.ActivePID() != 4242 {
		t.Errorf("ActivePID = %d, want 4242", h.ActivePID())
	}
	// The plugin-handshake env must be passed through.
	if !containsExact(f.env, "mlxba=adapter") {
		t.Error("plugin handshake env not passed to chrome")
	}
}

func TestHandle_StopClosesServer(t *testing.T) {
	f := newFakeChrome()
	opts := Options{Name: "prof-2", ProfileDir: t.TempDir(), Standalone: boolTruePtr(), SkipCDPDiscovery: true}
	h, err := launchStandaloneWith(opts, fakeDeps(f))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	cp, _ := argValue(f.args, "--client-port=")
	port, _ := strconv.Atoi(cp)

	// fp-server is live before Stop.
	if c, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second); derr == nil {
		c.Close()
	} else {
		t.Fatalf("fp-server not listening before Stop: %v", derr)
	}

	if err := h.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !f.killed {
		t.Error("chrome not killed on Stop")
	}
	// After Stop the fp-server must be closed.
	if c, derr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond); derr == nil {
		c.Close()
		t.Error("fp-server still listening after Stop")
	}
}

func TestHandle_IsActiveTracksPID(t *testing.T) {
	f := newFakeChrome()
	opts := Options{Name: "prof-3", ProfileDir: t.TempDir(), Standalone: boolTruePtr(), SkipCDPDiscovery: true}
	h, err := launchStandaloneWith(opts, fakeDeps(f))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer h.Stop()

	if active, _ := h.IsActive(); !active {
		t.Error("IsActive = false while process alive")
	}
	close(f.exit) // simulate chrome exit
	// wait goroutine flips the flag asynchronously.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if active, _ := h.IsActive(); !active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("IsActive stayed true after process exit")
}

func TestLaunchStandalone_LoadsExtensions(t *testing.T) {
	f := newFakeChrome()
	deps := fakeDeps(f)
	deps.resolveExtensions = func() []string { return []string{`C:\ext\a`, `C:\ext\b`} }

	opts := Options{Name: "prof-ext", ProfileDir: t.TempDir(), Standalone: boolTruePtr(), SkipCDPDiscovery: true}
	h, err := launchStandaloneWith(opts, deps)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer h.Stop()

	le, ok := argValue(f.args, "--load-extension=")
	if !ok {
		t.Fatal("--load-extension missing")
	}
	if le != `C:\ext\a,C:\ext\b` {
		t.Errorf("--load-extension = %q, want the two dirs joined by comma", le)
	}
}

func TestWaitCDPReady_ReturnsWhenServing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			_, _ = w.Write([]byte("[]"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	if err := waitCDPReady(srv.URL, 2*time.Second); err != nil {
		t.Errorf("waitCDPReady on live server = %v, want nil", err)
	}
}

func TestWaitCDPReady_TimesOut(t *testing.T) {
	// Nothing listening on this port → timeout quickly.
	if err := waitCDPReady("http://127.0.0.1:1", 300*time.Millisecond); err == nil {
		t.Error("waitCDPReady on dead endpoint = nil, want timeout error")
	}
}

func boolTruePtr() *bool { b := true; return &b }

func containsExact(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
