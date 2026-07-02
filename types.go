// Package mimickit is a self-owned Go module that launches Multilogin "mimic"
// (patched Chromium 149) browser profiles with NO cloud dependency. It wraps the
// adapter.exe go-plugin netrpc surface: spawn the adapter, hand it a fingerprint +
// masking flags, and get back a live CDP endpoint. The masking itself happens
// inside adapter.exe — this module only drives the RPC contract and resolves the
// locally installed core.
//
// Wire contract derived from adapter.exe DWARF debug info:
//   RPC service name on server: "Plugin"
//   Handshake: MagicCookieKey="mlxba" MagicCookieValue="adapter" ProtocolVersion=1
//   Protocol:  netrpc over yamux over TCP
package mimickit

// ── net/rpc request/response types ──────────────────────────────────────────

// StartRequest is the typed form of the Plugin.Start payload. FpRaw carries the
// JSON-marshaled ReadyProfileResponse (NOT a flat ReadyProfile).
type StartRequest struct {
	ProfileID      string
	ProfileDirPath string
	Selenium       bool
	Puppeteer      bool
	Headless       bool
	FpRaw          []byte
	Overrides      *LauncherOverrides
}

// StartResponse.Value is the ChromeDriver port the adapter reports back. The real
// CDP debug port is discovered separately by scanning the chrome process cmdline.
type StartResponse struct {
	Value string
}

// LauncherOverrides carries per-launch runtime overrides applied by the adapter.
type LauncherOverrides struct {
	Timezone   string
	PublicIP   string
	Coordinate Geo
	Proxy      *Proxy
}

type Geo struct {
	Latitude  float32
	Longitude float32
	Altitude  float32
	Accuracy  float32
}

// Proxy is the flat proxy spec inside LauncherOverrides (distinct from CoreProxy).
type Proxy struct {
	Type        string
	Host        string
	Port        int
	Username    string
	Password    string
	SaveTraffic bool
}

// ── ReadyProfileResponse — the shape FpRaw must decode to ───────────────────
// adapter.exe prepareFingerprint decodes FpRaw into ReadyProfileResponse and
// checks .Data != nil. A flat ReadyProfile without this wrapper causes "invalid fp".

type ReadyProfileResponse struct {
	Data   *ReadyProfile  `json:"data"`
	Status ResponseStatus `json:"status"`
}

type ResponseStatus struct {
	ErrorCode string `json:"error_code"`
	HTTPCode  int    `json:"http_code"`
	Message   string `json:"message"`
}

// ── ReadyProfile — JSON shape of StartRequest.FpRaw ─────────────────────────
// Field names are the adapter.exe binary JSON struct tags (snake_case).

type ReadyProfile struct {
	Core        ReadyProfileCore `json:"core"`
	Fingerprint FingerprintData  `json:"fingerprint"`
	Flags       ProfileMetaFlags `json:"flags"`
	IsLocal     bool             `json:"is_local"`
	Name        string           `json:"name"`
	FolderID    string           `json:"folder_id"`

	BrowserExtensions *[]BrowserExtension `json:"browser_extensions,omitempty"`
	CustomDns         *string             `json:"custom_dns,omitempty"`
	CustomStartUrls   *[]string           `json:"custom_start_urls,omitempty"`
}

type ReadyProfileCore struct {
	Browser                          ProfileMetaCore `json:"browser"`
	BrowserVersion                   string          `json:"browser_version"`
	AutoUpdateCore                   bool            `json:"auto_update_core"`
	AbortStartIfProxyLeaks           *bool           `json:"abort_start_if_proxy_leaks,omitempty"`
	GeolocationFillBasedOnExternalIP *bool           `json:"geolocation_fill_based_on_external_ip,omitempty"`
	TimezoneFillBasedOnExternalIP    *bool           `json:"timezone_fill_based_on_external_ip,omitempty"`
	Proxy                            *CoreProxy      `json:"proxy,omitempty"`
}

type ProfileMetaCore struct {
	BrowserType      string `json:"browser_type"`
	CoreVersion      int    `json:"core_version"`
	CoreMinorVersion *int   `json:"core_minor_version,omitempty"`
	OsType           string `json:"os_type"`
}

// CoreProxy is the openapi-style proxy (pointer fields), distinct from Proxy.
type CoreProxy struct {
	Type        *string `json:"type,omitempty"`
	Host        string  `json:"host"`
	Port        *int    `json:"port,omitempty"`
	Username    *string `json:"username,omitempty"`
	Password    *string `json:"password,omitempty"`
	SaveTraffic *bool   `json:"save_traffic,omitempty"`
}

type FingerprintData struct {
	Fonts          *[]string       `json:"fonts,omitempty"`
	Geolocation    *FPGeolocation  `json:"geolocation,omitempty"`
	Graphic        *FPGraphic      `json:"graphic,omitempty"`
	Localization   *FPLocalization `json:"localization,omitempty"`
	MaskingOptions *MaskingOptions `json:"masking_options,omitempty"`
	MediaDevices   *FPMediaDevices `json:"media_devices,omitempty"`
	Navigator      *FPNavigator    `json:"navigator,omitempty"`
	Ports          *[]int          `json:"ports,omitempty"`
	Screen         *FPScreen       `json:"screen,omitempty"`
	Timezone       *FPTimezone     `json:"timezone,omitempty"`
	Webrtc         *FPWebrtc       `json:"webrtc,omitempty"`
}

type FPNavigator struct {
	HardwareConcurrency int     `json:"hardware_concurrency"`
	Platform            string  `json:"platform"`
	UserAgent           string  `json:"user_agent"`
	MaxTouchPoints      *int    `json:"max_touch_points,omitempty"`
	OsCPU               *string `json:"os_cpu,omitempty"`
	PlatformVersion     *string `json:"platform_version,omitempty"`
}

type FPScreen struct {
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	PixelRatio float64 `json:"pixel_ratio"`
}

type FPGraphic struct {
	Vendor   string  `json:"vendor"`
	Renderer string  `json:"renderer"`
	VendorID *string `json:"vendor_id,omitempty"`
	DeviceID *string `json:"device_id,omitempty"`
}

type FPGeolocation struct {
	Latitude  float32 `json:"latitude"`
	Longitude float32 `json:"longitude"`
	Altitude  float32 `json:"altitude"`
	Accuracy  float32 `json:"accuracy"`
}

type FPLocalization struct {
	AcceptLanguages string `json:"accept_languages"`
	Languages       string `json:"languages"`
	Locale          string `json:"locale"`
}

type FPMediaDevices struct {
	AudioInputs  int `json:"audio_inputs"`
	AudioOutputs int `json:"audio_outputs"`
	VideoInputs  int `json:"video_inputs"`
}

type FPTimezone struct {
	Zone string `json:"zone"`
}

type FPWebrtc struct {
	PublicIP string `json:"public_ip"`
}

type MaskingOptions struct {
	UseCustomMobileDevice *bool `json:"use_custom_mobile_device,omitempty"`
}

// ProfileMetaFlags is the masking configuration (string enums per adapter.exe).
type ProfileMetaFlags struct {
	AudioMasking        string  `json:"audio_masking"`
	CanvasNoise         *string `json:"canvas_noise,omitempty"`
	FontsMasking        string  `json:"fonts_masking"`
	GeolocationMasking  string  `json:"geolocation_masking"`
	GeolocationPopup    string  `json:"geolocation_popup"`
	GraphicsMasking     string  `json:"graphics_masking"`
	GraphicsNoise       string  `json:"graphics_noise"`
	LocalizationMasking string  `json:"localization_masking"`
	MediaDevicesMasking string  `json:"media_devices_masking"`
	NavigatorMasking    string  `json:"navigator_masking"`
	PortsMasking        string  `json:"ports_masking"`
	ProxyMasking        string  `json:"proxy_masking"`
	QuicMode            *string `json:"quic_mode,omitempty"`
	ScreenMasking       string  `json:"screen_masking"`
	StartupBehavior     *string `json:"startup_behavior,omitempty"`
	TimezoneMasking     string  `json:"timezone_masking"`
	WebrtcMasking       string  `json:"webrtc_masking"`
}

type BrowserExtension struct {
	ID    *string `json:"id,omitempty"`
	Name  string  `json:"name"`
	Store string  `json:"store"`
	URL   string  `json:"url"`
}
