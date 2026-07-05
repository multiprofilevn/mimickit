package standalone

import (
	"strings"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func hasArg(args []string, prefix string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func TestBuildChromeArgs_UserArgsLast(t *testing.T) {
	cfg := Config{
		SessionID:   "sid",
		UserDataDir: `C:\data`,
		ClientPort:  1111,
		ChromeArgs:  []string{"--foo", "--proxy-server=x"},
	}
	args := BuildChromeArgs(cfg)
	n := len(args)
	if args[n-2] != "--foo" || args[n-1] != "--proxy-server=x" {
		t.Errorf("last two args = %q, want [--foo --proxy-server=x]", args[n-2:])
	}
}

func TestBuildChromeArgs_ManagedFalseMinimal(t *testing.T) {
	cfg := Config{
		SessionID:           "sid",
		UserDataDir:         `C:\data`,
		ClientPort:          1111,
		RemoteDebuggingPort: 2222,
		ManagedFlags:        boolPtr(false),
		ChromeArgs:          []string{"--extra"},
	}
	args := BuildChromeArgs(cfg)
	// Minimal: exactly the 4 required flags + the 1 user arg.
	if len(args) != 5 {
		t.Fatalf("minimal args count = %d, want 5: %v", len(args), args)
	}
	for _, want := range []string{"--client-port=", "--client-session-id=", "--user-data-dir=", "--remote-debugging-port="} {
		if !hasArg(args, want) {
			t.Errorf("minimal set missing %s", want)
		}
	}
	if hasArg(args, "--user-agent=") {
		t.Error("minimal set must not include --user-agent")
	}
}

func TestBuildChromeArgs_ManagedTrueFull(t *testing.T) {
	cfg := Config{
		SessionID:           "sid",
		SessionName:         "prof",
		UserAgent:           "UA/149",
		Platform:            "Win32",
		PlatformVersion:     "15.0.0",
		AcceptLanguage:      "en-US,en;q=0.9",
		Languages:           "en-US,en",
		Width:               1920,
		Height:              1080,
		ClientPort:          1111,
		RemoteDebuggingPort: 2222,
		PortWhitelist:       []int{80, 443},
	}
	args := BuildChromeArgs(cfg) // ManagedFlags nil => full set
	for _, want := range []string{"--user-agent=UA/149", "--client-port=1111", "--remote-debugging-port=2222", "--disable-quic", "--primary_language=en-US"} {
		if !hasArg(args, want) {
			t.Errorf("full set missing %s in %v", want, args)
		}
	}
}

func TestBuildChromeArgs_LoadExtensions(t *testing.T) {
	cfg := Config{
		SessionID:      "sid",
		UserDataDir:    `C:\data`,
		ClientPort:     1111,
		LoadExtensions: []string{`C:\ext\a`, `C:\ext\b`},
		ChromeArgs:     []string{"--last"},
	}
	args := buildArgsAndCheck(t, cfg)
	if args != `C:\ext\a,C:\ext\b` {
		t.Errorf("--load-extension = %q, want both dirs comma-joined", args)
	}

	// No extensions => no flag.
	if hasArg(BuildChromeArgs(Config{SessionID: "s", ClientPort: 1}), "--load-extension=") {
		t.Error("--load-extension present with no LoadExtensions")
	}
}

func buildArgsAndCheck(t *testing.T, cfg Config) string {
	t.Helper()
	args := BuildChromeArgs(cfg)
	// --load-extension must come before the trailing user args.
	le := ""
	for i, a := range args {
		if strings.HasPrefix(a, "--load-extension=") {
			le = strings.TrimPrefix(a, "--load-extension=")
			if i == len(args)-1 && len(cfg.ChromeArgs) > 0 {
				t.Error("--load-extension is last; user ChromeArgs should follow it")
			}
		}
	}
	if args[len(args)-1] != "--last" {
		t.Errorf("last arg = %q, want user arg --last", args[len(args)-1])
	}
	return le
}

func TestEnvHandshake_SetsFour(t *testing.T) {
	env := PluginHandshakeEnv()
	want := map[string]bool{
		"mlxba=adapter":              false,
		"PLUGIN_MIN_PORT=10000":      false,
		"PLUGIN_MAX_PORT=25000":      false,
		"PLUGIN_PROTOCOL_VERSIONS=1": false,
	}
	if len(env) != 4 {
		t.Fatalf("env count = %d, want 4: %v", len(env), env)
	}
	for _, e := range env {
		if _, ok := want[e]; !ok {
			t.Errorf("unexpected env %q", e)
		}
		want[e] = true
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing env %q", k)
		}
	}
}
