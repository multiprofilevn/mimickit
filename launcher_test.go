package mimickit

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAssembleReadyProfile_DefaultsDecodeAsWrapper verifies FpRaw marshals to the
// ReadyProfileResponse shape adapter.exe expects: .data != nil with a mimic core.
func TestAssembleReadyProfile_DefaultsDecodeAsWrapper(t *testing.T) {
	opts := Options{Name: "poc", OSType: "windows"}
	raw, err := assembleReadyProfile("", opts, resolveFingerprint(opts))
	if err != nil {
		t.Fatalf("assembleReadyProfile: %v", err)
	}

	var wrapper ReadyProfileResponse
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal FpRaw: %v", err)
	}
	if wrapper.Data == nil {
		t.Fatal("wrapper.Data is nil — adapter would reject with 'invalid fp'")
	}
	if got := wrapper.Data.Core.Browser.BrowserType; got != "mimic" {
		t.Errorf("BrowserType = %q, want mimic", got)
	}
	if wrapper.Data.Core.Browser.CoreVersion <= 0 {
		t.Errorf("CoreVersion = %d, want > 0", wrapper.Data.Core.Browser.CoreVersion)
	}
	if wrapper.Data.Fingerprint.Navigator == nil {
		t.Error("default fingerprint missing Navigator")
	}
	if wrapper.Data.Flags.AudioMasking != "natural" {
		t.Errorf("AudioMasking = %q, want natural (green config)", wrapper.Data.Flags.AudioMasking)
	}
}

// TestAssembleReadyProfile_CallerFingerprintFlags verifies API-supplied
// fingerprint + flags override the POC defaults while core stays launcher-set.
func TestAssembleReadyProfile_CallerFingerprintFlags(t *testing.T) {
	fp, _ := json.Marshal(FingerprintData{
		Navigator: &FPNavigator{Platform: "MacIntel", UserAgent: "custom-ua"},
		Screen:    &FPScreen{Width: 2560, Height: 1440, PixelRatio: 2.0},
	})
	flags, _ := json.Marshal(ProfileMetaFlags{AudioMasking: "mask", NavigatorMasking: "mask"})

	callerOpts := Options{
		Name:        "caller",
		OSType:      "macos",
		Fingerprint: fp,
		Flags:       flags,
	}
	raw, err := assembleReadyProfile("", callerOpts, resolveFingerprint(callerOpts))
	if err != nil {
		t.Fatalf("assembleReadyProfile: %v", err)
	}
	var wrapper ReadyProfileResponse
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wrapper.Data.Fingerprint.Navigator.UserAgent != "custom-ua" {
		t.Errorf("caller UA not applied: %q", wrapper.Data.Fingerprint.Navigator.UserAgent)
	}
	if wrapper.Data.Flags.AudioMasking != "mask" {
		t.Errorf("caller flag not applied: %q", wrapper.Data.Flags.AudioMasking)
	}
	// Core stays launcher-owned regardless of caller OSType mapping.
	if wrapper.Data.Core.Browser.OsType != "MacIntel" {
		t.Errorf("OsType = %q, want MacIntel", wrapper.Data.Core.Browser.OsType)
	}
	if wrapper.Data.Core.Browser.BrowserType != "mimic" {
		t.Errorf("BrowserType = %q, want mimic", wrapper.Data.Core.Browser.BrowserType)
	}
}

func TestStartRequestProxyOverride(t *testing.T) {
	ov := buildOverrides(Options{Proxy: &ProxySpec{Type: "socks5", Host: "1.2.3.4", Port: 1080, Username: "u", Password: "p"}}, FingerprintData{})
	if ov.Proxy == nil {
		t.Fatal("proxy override not wired")
	}
	if ov.Proxy.Host != "1.2.3.4" || ov.Proxy.Port != 1080 || ov.Proxy.Type != "socks5" {
		t.Errorf("proxy override mismatch: %+v", ov.Proxy)
	}
}

// TestAssembleReadyProfile_ProxyForcesCustomMasking guards the proxy-leak fix:
// when a proxy is set, proxy_masking MUST be "custom" (adapter only wires the
// proxy into chrome for "custom"; "disabled" leaks the real IP), even when the
// caller-supplied flags say "disabled".
func TestAssembleReadyProfile_ProxyForcesCustomMasking(t *testing.T) {
	flags, _ := json.Marshal(ProfileMetaFlags{ProxyMasking: "disabled"})

	withProxy := Options{
		Name:  "with-proxy",
		Flags: flags,
		Proxy: &ProxySpec{Type: "http", Host: "1.2.3.4", Port: 8080},
	}
	raw, err := assembleReadyProfile("", withProxy, resolveFingerprint(withProxy))
	if err != nil {
		t.Fatalf("assembleReadyProfile: %v", err)
	}
	var wrapper ReadyProfileResponse
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if wrapper.Data.Flags.ProxyMasking != "custom" {
		t.Errorf("ProxyMasking with proxy = %q, want custom", wrapper.Data.Flags.ProxyMasking)
	}

	// No proxy → caller flag preserved (no forced override, direct connection).
	noProxy := Options{Name: "no-proxy", Flags: flags}
	raw2, err := assembleReadyProfile("", noProxy, resolveFingerprint(noProxy))
	if err != nil {
		t.Fatalf("assembleReadyProfile (no proxy): %v", err)
	}
	var wrapper2 ReadyProfileResponse
	if err := json.Unmarshal(raw2, &wrapper2); err != nil {
		t.Fatalf("unmarshal (no proxy): %v", err)
	}
	if wrapper2.Data.Flags.ProxyMasking != "disabled" {
		t.Errorf("ProxyMasking without proxy = %q, want disabled", wrapper2.Data.Flags.ProxyMasking)
	}
}

func TestProfileIDFromName_UUIDFormat(t *testing.T) {
	id := profileIDFromName("my-profile")
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		t.Errorf("profileID %q is not UUID-shaped", id)
	}
	if id != profileIDFromName("my-profile") {
		t.Error("profileID not deterministic")
	}
}

func TestParseDebugPort(t *testing.T) {
	cases := map[string]int{
		`chrome.exe --remote-debugging-port=12345 --foo`: 12345,
		`chrome --foo --remote-debugging-port=9222`:      9222,
		`chrome.exe --remote-debugging-port=0`:           0,
		`chrome.exe --no-debug`:                          0,
	}
	for cmd, want := range cases {
		if got := parseDebugPort(cmd); got != want {
			t.Errorf("parseDebugPort(%q) = %d, want %d", cmd, got, want)
		}
	}
}

func TestCoreVersions_FallbackAndParse(t *testing.T) {
	// Non-existent explicit dir → CoreVersion "" → fallback (149, "149").
	major, bv := coreVersions(`Z:\definitely\not\here`)
	if major != 149 || bv != "149" {
		t.Errorf("fallback coreVersions = (%d, %q), want (149, 149)", major, bv)
	}
}
