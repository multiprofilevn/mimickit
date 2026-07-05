package mimickit

import (
	"fmt"

	"github.com/tuwibu/mimickit/internal/standalone"
)

// MapFingerprint translates the API-supplied FingerprintData + ProfileMetaFlags
// into the flat standalone.Config the adapter-less launcher consumes. It is a
// pure function (no I/O, no side effects) so the RE-critical masking mapping is
// fully unit-testable against golden fixtures. Identity/port fields (SessionID,
// ClientPort, UserDataDir, ...) are filled by the launcher, not here.
//
// When proxy is non-nil, its chrome flags are emitted into Config.ChromeArgs
// (before any user args the launcher appends) and proxy_masking is forced to
// "custom" — the one masking mode under which mimic chrome actually routes the
// proxy instead of leaking the real IP.
//
// fonts and media_devices are intentionally NOT emitted as fp-server params:
// with fonts_masking / media_devices_masking = "natural" the browser exposes the
// real values, so there is nothing to inject. This matches the RE'd param set.
func MapFingerprint(fp FingerprintData, flags ProfileMetaFlags, proxy *ProxySpec) standalone.Config {
	cfg := standalone.Config{
		Platform:        "Win32",
		PlatformVersion: "15.0.0",
	}

	if nav := fp.Navigator; nav != nil {
		cfg.UserAgent = nav.UserAgent
		if nav.Platform != "" {
			cfg.Platform = nav.Platform
		}
		if nav.PlatformVersion != nil && *nav.PlatformVersion != "" {
			cfg.PlatformVersion = *nav.PlatformVersion
		}
		cfg.HWConcurrency = nav.HardwareConcurrency
	}
	if g := fp.Graphic; g != nil {
		cfg.GLVendor = g.Vendor
		cfg.GLRenderer = g.Renderer
	}
	if s := fp.Screen; s != nil {
		cfg.Width = s.Width
		cfg.Height = s.Height
	}
	if tz := fp.Timezone; tz != nil {
		cfg.Timezone = tz.Zone
	}
	if loc := fp.Localization; loc != nil {
		cfg.AcceptLanguage = loc.AcceptLanguages
		cfg.Languages = loc.Languages
	}
	if geo := fp.Geolocation; geo != nil {
		cfg.Geo = &standalone.ConfigGeo{
			Lat:  float64(geo.Latitude),
			Long: float64(geo.Longitude),
			Acc:  float64(geo.Accuracy),
			Type: 3,
		}
	}
	if fp.Ports != nil {
		cfg.PortWhitelist = append([]int(nil), *fp.Ports...)
	}

	cfg.SessionFlags = mapSessionFlags(flags)

	if proxy != nil {
		cfg.SessionFlags["proxy_masking"] = "custom"
		cfg.ChromeArgs = mapProxyArgs(proxy)
	}

	return cfg
}

// mapSessionFlags flattens ProfileMetaFlags into the string map the fp-server
// encrypts as "session-flags". Optional (pointer) flags are included only when
// present. If the struct is empty (caller omitted flags), the verified green
// defaults are used so masking never falls back to "no flags".
func mapSessionFlags(flags ProfileMetaFlags) map[string]string {
	if flags.NavigatorMasking == "" && flags.ScreenMasking == "" && flags.GraphicsMasking == "" {
		flags = defaultGreenFlags()
	}
	m := map[string]string{
		"audio_masking":         flags.AudioMasking,
		"fonts_masking":         flags.FontsMasking,
		"geolocation_masking":   flags.GeolocationMasking,
		"geolocation_popup":     flags.GeolocationPopup,
		"graphics_masking":      flags.GraphicsMasking,
		"graphics_noise":        flags.GraphicsNoise,
		"localization_masking":  flags.LocalizationMasking,
		"media_devices_masking": flags.MediaDevicesMasking,
		"navigator_masking":     flags.NavigatorMasking,
		"ports_masking":         flags.PortsMasking,
		"proxy_masking":         flags.ProxyMasking,
		"screen_masking":        flags.ScreenMasking,
		"timezone_masking":      flags.TimezoneMasking,
		"webrtc_masking":        flags.WebrtcMasking,
	}
	if flags.CanvasNoise != nil {
		m["canvas_noise"] = *flags.CanvasNoise
	}
	if flags.QuicMode != nil {
		m["quic_mode"] = *flags.QuicMode
	}
	if flags.StartupBehavior != nil {
		m["startup_behavior"] = *flags.StartupBehavior
	}
	return m
}

// mapProxyArgs renders a ProxySpec into the chrome proxy flags mimic chrome reads
// under proxy_masking="custom". Username/password are emitted as separate flags
// (mimic chrome's extension of --proxy-server), matching the adapter path.
func mapProxyArgs(p *ProxySpec) []string {
	scheme := p.Type
	if scheme == "" {
		scheme = "http"
	}
	args := []string{fmt.Sprintf("--proxy-server=%s://%s:%d", scheme, p.Host, p.Port)}
	if p.Username != "" {
		args = append(args, "--proxy-username="+p.Username)
	}
	if p.Password != "" {
		args = append(args, "--proxy-password="+p.Password)
	}
	return args
}
