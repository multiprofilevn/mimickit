//go:build mimic_e2e

// Integration test that requires a REAL adapter.exe + patched chrome core.
// Run with:  go test -tags mimic_e2e -run E2E -v ./...
// Point at a core via MLX_ADAPTER_DIR if it is not auto-discovered.
package mimickit

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestE2E_LaunchProfileGetsCDP(t *testing.T) {
	if !CoreExists() {
		t.Skipf("no mimic core resolvable (set MLX_ADAPTER_DIR); ResolveCoreDir=%q", ResolveCoreDir(""))
	}

	profileBase, err := os.MkdirTemp("", "mimickit-e2e-")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(profileBase)

	h, err := Launch(Options{
		Name:       "e2e-poc",
		OSType:     "windows",
		ProfileDir: profileBase,
		Puppeteer:  true,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	t.Cleanup(func() { _ = h.Stop() })

	if h.CDPPort <= 0 {
		t.Fatalf("no CDP port (POC gate #2 FAILED — Puppeteer did not open --remote-debugging-port)")
	}
	t.Logf("CDP endpoint: %s (chrome pid %d, driver %q)", h.CDPAddress, h.ActivePID(), h.DriverPort)

	// Assert /json/version answers 200 (POC gate #2 PASS = CDP, not just webdriver).
	client := &http.Client{Timeout: 5 * time.Second}
	var ok bool
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(h.CDPAddress + "/json/version")
		if err == nil && resp.StatusCode == 200 {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			var v map[string]any
			_ = json.Unmarshal(body, &v)
			t.Logf("/json/version: %v", v)
			ok = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ok {
		t.Fatal("/json/version did not return 200 — CDP endpoint not usable")
	}

	active, err := h.IsActive()
	if err != nil {
		t.Errorf("IsActive: %v", err)
	}
	if !active {
		t.Error("IsActive=false while browser should be running")
	}
}

// TestE2E_ProxyAccepted is POC gate #3. It requires a live proxy supplied via
// env (MLX_TEST_PROXY_HOST/PORT[/TYPE/USER/PASS]). It asserts the adapter
// ACCEPTS a proxy-configured Start and still yields a CDP endpoint. Full
// exit-IP verification (browsing through the proxy) is a manual follow-up:
// connect to h.CDPAddress and navigate to an IP-echo service.
func TestE2E_ProxyAccepted(t *testing.T) {
	host := os.Getenv("MLX_TEST_PROXY_HOST")
	if host == "" || !CoreExists() {
		t.Skip("set MLX_TEST_PROXY_HOST (+PORT) and a resolvable core to run the proxy gate")
	}
	port, err := strconv.Atoi(os.Getenv("MLX_TEST_PROXY_PORT"))
	if err != nil || port == 0 {
		t.Skip("MLX_TEST_PROXY_PORT missing/invalid")
	}
	typ := os.Getenv("MLX_TEST_PROXY_TYPE")
	if typ == "" {
		typ = "http"
	}

	base, _ := os.MkdirTemp("", "mimickit-proxy-")
	defer os.RemoveAll(base)

	h, lerr := Launch(Options{
		Name:       "e2e-proxy",
		ProfileDir: base,
		Puppeteer:  true,
		Proxy: &ProxySpec{
			Type: typ, Host: host, Port: port,
			Username: os.Getenv("MLX_TEST_PROXY_USER"),
			Password: os.Getenv("MLX_TEST_PROXY_PASS"),
		},
	})
	if lerr != nil {
		t.Fatalf("Launch with proxy: %v", lerr)
	}
	t.Cleanup(func() { _ = h.Stop() })
	if h.CDPPort <= 0 {
		t.Fatal("proxy-configured launch produced no CDP port")
	}
	t.Logf("proxy gate: adapter accepted proxy %s:%d, CDP at %s", host, port, h.CDPAddress)
}
