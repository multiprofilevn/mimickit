package mimickit

import (
	"strings"
	"testing"
)

func TestMapFingerprint_FromFixture(t *testing.T) {
	for name, want := range expectedParams {
		fx := loadConfigureFixture(t, name)
		cfg := MapFingerprint(fx.Fingerprint, fx.Flags, nil)

		if cfg.UserAgent != want.userAgent {
			t.Errorf("%s: UserAgent = %q, want %q", name, cfg.UserAgent, want.userAgent)
		}
		if cfg.Platform != want.platform {
			t.Errorf("%s: Platform = %q, want %q", name, cfg.Platform, want.platform)
		}
		if cfg.GLVendor != want.glVendor {
			t.Errorf("%s: GLVendor = %q, want %q", name, cfg.GLVendor, want.glVendor)
		}
		if cfg.GLRenderer != want.glRenderer {
			t.Errorf("%s: GLRenderer = %q, want %q", name, cfg.GLRenderer, want.glRenderer)
		}
		if cfg.Width != want.width || cfg.Height != want.height {
			t.Errorf("%s: screen = %dx%d, want %dx%d", name, cfg.Width, cfg.Height, want.width, want.height)
		}
		if cfg.HWConcurrency != want.hwConcurrency {
			t.Errorf("%s: HWConcurrency = %d, want %d", name, cfg.HWConcurrency, want.hwConcurrency)
		}
		if cfg.Timezone != want.timezone {
			t.Errorf("%s: Timezone = %q, want %q", name, cfg.Timezone, want.timezone)
		}
		if cfg.Languages != want.languages {
			t.Errorf("%s: Languages = %q, want %q", name, cfg.Languages, want.languages)
		}
		if cfg.SessionFlags["proxy_masking"] != want.proxyMasking {
			t.Errorf("%s: proxy_masking = %q, want %q", name, cfg.SessionFlags["proxy_masking"], want.proxyMasking)
		}
	}
}

func TestMapSessionFlags_GreenTable(t *testing.T) {
	flags := defaultGreenFlags()
	m := mapSessionFlags(flags)
	checks := map[string]string{
		"navigator_masking": "mask",
		"screen_masking":    "mask",
		"audio_masking":     "natural",
		"proxy_masking":     "disabled",
		"quic_mode":         "disabled",
		"startup_behavior":  "recover",
	}
	for k, v := range checks {
		if m[k] != v {
			t.Errorf("sessionFlags[%q] = %q, want %q", k, m[k], v)
		}
	}
}

func TestMapProxyArgs_CustomMasking(t *testing.T) {
	fx := loadConfigureFixture(t, "configure-multilogin-with-proxy.json")
	proxy := &ProxySpec{Type: "http", Host: "1.2.3.4", Port: 8080, Username: "u", Password: "p"}
	cfg := MapFingerprint(fx.Fingerprint, fx.Flags, proxy)

	if !containsPrefix(cfg.ChromeArgs, "--proxy-server=http://1.2.3.4:8080") {
		t.Errorf("with-proxy: missing --proxy-server in %v", cfg.ChromeArgs)
	}
	if !containsPrefix(cfg.ChromeArgs, "--proxy-username=u") {
		t.Errorf("with-proxy: missing --proxy-username")
	}
	if cfg.SessionFlags["proxy_masking"] != "custom" {
		t.Errorf("with-proxy: proxy_masking = %q, want custom", cfg.SessionFlags["proxy_masking"])
	}

	// No-proxy fixture, no ProxySpec => no proxy args.
	nf := loadConfigureFixture(t, "configure-multilogin-no-proxy.json")
	ncfg := MapFingerprint(nf.Fingerprint, nf.Flags, nil)
	if containsPrefix(ncfg.ChromeArgs, "--proxy-server=") {
		t.Errorf("no-proxy: unexpected proxy args %v", ncfg.ChromeArgs)
	}
}

func TestMapFingerprint_NoFieldDropped(t *testing.T) {
	fx := loadConfigureFixture(t, "configure-multilogin-no-proxy.json")
	cfg := MapFingerprint(fx.Fingerprint, fx.Flags, nil)
	if cfg.UserAgent == "" || cfg.GLVendor == "" || cfg.GLRenderer == "" ||
		cfg.Width == 0 || cfg.Height == 0 || cfg.HWConcurrency == 0 ||
		cfg.Timezone == "" || cfg.Languages == "" || len(cfg.PortWhitelist) == 0 || cfg.Geo == nil {
		t.Errorf("a required field was dropped: %+v", cfg)
	}
}

func TestMapFingerprint_MissingFieldFallback(t *testing.T) {
	fx := loadConfigureFixture(t, "configure-multilogin-no-proxy.json")
	// Empty flags => should fall back to green defaults, not panic or blank map.
	cfg := MapFingerprint(fx.Fingerprint, ProfileMetaFlags{}, nil)
	if cfg.SessionFlags["navigator_masking"] != "mask" {
		t.Errorf("empty flags did not fall back to green defaults: %v", cfg.SessionFlags)
	}
}

func containsPrefix(xs []string, prefix string) bool {
	for _, x := range xs {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}
