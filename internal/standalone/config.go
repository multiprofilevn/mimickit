package standalone

// Config is the flat, already-resolved launch description the standalone
// mechanism consumes. The root package maps an API FingerprintData +
// ProfileMetaFlags into this shape (fingerprint_map.go); this package never sees
// the nested API types, keeping it a leaf. Every field here is a value chrome or
// the fp-server needs directly — no further masking logic downstream.
type Config struct {
	// Identity / paths.
	SessionID   string // UUID-format --client-session-id
	SessionName string // --session_name
	UserDataDir string // --user-data-dir

	// Ports (0 => auto-allocated at launch).
	ClientPort          int // fp-server port chrome fetches the fingerprint from
	RemoteDebuggingPort int // CDP port

	// Fingerprint params (already flattened from FingerprintData).
	UserAgent       string
	Platform        string
	PlatformVersion string
	GLVendor        string
	GLRenderer      string
	Width           int
	Height          int
	HWConcurrency   int
	Timezone        string
	Languages       string // "en-US,en"
	AcceptLanguage  string // "en-US,en;q=0.9"
	Geo             *ConfigGeo
	PortWhitelist   []int
	RTCBlock        *bool

	// Masking flags (ProfileMetaFlags flattened to the fp-server's string map).
	SessionFlags map[string]string
	// ExtraFingerprint injects/overrides raw fp-server params last.
	ExtraFingerprint map[string]string

	// LoadExtensions are unpacked-extension dirs passed via --load-extension. The
	// root resolves them from CustomExtensionsDir (adapter-less: chrome is spawned
	// directly, so there is no adapter to scan the folder). Empty => no flag.
	LoadExtensions []string

	// Chrome argument control.
	ChromeArgs   []string // arbitrary user passthrough, appended LAST (override)
	ManagedFlags *bool    // nil/true => full managed mimic flag set; false => minimal
}

// ConfigGeo is the flat geolocation the fp-server encodes.
type ConfigGeo struct {
	Lat  float64
	Long float64
	Acc  float64
	Type int
}
