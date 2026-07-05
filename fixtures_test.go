package mimickit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// configureFixture mirrors the fingerprint+flags portion of a real
// /configure/multilogin response. It parses into the SAME structs the launcher
// consumes (FingerprintData, ProfileMetaFlags) — no redefined shapes. The
// fixtures under testdata/ are the golden source of truth the standalone masking
// mapper (fingerprint_map.go) is tested against; real pixelscan/creepjs parity is
// a manual gate, not automatable here.
type configureFixture struct {
	Fingerprint FingerprintData  `json:"fingerprint"`
	Flags       ProfileMetaFlags `json:"flags"`
}

// loadConfigureFixture reads and parses a testdata fixture, failing the test on
// any I/O or decode error.
func loadConfigureFixture(t *testing.T, name string) configureFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var fx configureFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return fx
}

// expectedFieldSet is the flat set of masked values the standalone mapper must
// derive from a fixture's FingerprintData. Declared here so phase-4's mapping
// test (fingerprint_map_test.go) asserts against the SAME expectations the
// fixtures encode — one contract, two consumers.
type expectedFieldSet struct {
	userAgent     string
	platform      string
	glVendor      string
	glRenderer    string
	width         int
	height        int
	hwConcurrency int
	timezone      string
	languages     string
	proxyMasking  string
}

// expectedParams maps each fixture file to the flat params the mapper must emit.
// Kept in sync with the testdata JSON by hand; a drift surfaces as a phase-4
// mapping-test failure.
var expectedParams = map[string]expectedFieldSet{
	"configure-multilogin-no-proxy.json": {
		userAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		platform:      "Win32",
		glVendor:      "Google Inc. (NVIDIA)",
		glRenderer:    "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
		width:         1920,
		height:        1080,
		hwConcurrency: 8,
		timezone:      "America/New_York",
		languages:     "en-US,en",
		proxyMasking:  "disabled",
	},
	"configure-multilogin-with-proxy.json": {
		userAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		platform:      "Win32",
		glVendor:      "Google Inc. (Intel)",
		glRenderer:    "ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0, D3D11)",
		width:         2560,
		height:        1440,
		hwConcurrency: 12,
		timezone:      "Europe/Berlin",
		languages:     "de-DE,de,en",
		proxyMasking:  "custom",
	},
}

func TestLoadFixture_ShapeValid(t *testing.T) {
	for name := range expectedParams {
		fx := loadConfigureFixture(t, name)

		if fx.Fingerprint.Navigator == nil || fx.Fingerprint.Navigator.UserAgent == "" {
			t.Errorf("%s: navigator.user_agent empty", name)
		}
		if fx.Fingerprint.Graphic == nil || fx.Fingerprint.Graphic.Vendor == "" || fx.Fingerprint.Graphic.Renderer == "" {
			t.Errorf("%s: graphic.vendor/renderer empty", name)
		}
		if fx.Fingerprint.Screen == nil || fx.Fingerprint.Screen.Width == 0 || fx.Fingerprint.Screen.Height == 0 {
			t.Errorf("%s: screen.width/height missing", name)
		}
		if fx.Fingerprint.Timezone == nil || fx.Fingerprint.Timezone.Zone == "" {
			t.Errorf("%s: timezone.zone empty", name)
		}
	}
}

func TestFixture_ProxyFlagOverride(t *testing.T) {
	withProxy := loadConfigureFixture(t, "configure-multilogin-with-proxy.json")
	if withProxy.Flags.ProxyMasking != "custom" {
		t.Errorf("with-proxy fixture: proxy_masking = %q, want custom", withProxy.Flags.ProxyMasking)
	}

	noProxy := loadConfigureFixture(t, "configure-multilogin-no-proxy.json")
	if noProxy.Flags.ProxyMasking != "disabled" {
		t.Errorf("no-proxy fixture: proxy_masking = %q, want disabled", noProxy.Flags.ProxyMasking)
	}
}
