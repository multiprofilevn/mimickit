package mimickit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// cdpDiscoveryTimeout bounds how long Launch waits for chrome to expose its
// --remote-debugging-port after the adapter reports Start success.
const cdpDiscoveryTimeout = 15 * time.Second

// ProxySpec is the per-profile proxy the browser should route through. It maps
// onto the adapter's LauncherOverrides.Proxy.
type ProxySpec struct {
	Type        string // "http" | "socks5" | ...
	Host        string
	Port        int
	Username    string
	Password    string
	SaveTraffic bool
}

// Options configures a single profile launch.
type Options struct {
	Name       string // profile name — also seeds the deterministic ProfileID
	OSType     string // "windows" | "macos" | "linux"; default "windows"
	ProfileDir string // base dir; profile data stored under ProfileDir/<name>
	AdapterDir string // dir containing adapter.exe; "" => ResolveCoreDir
	Headless   bool
	Puppeteer  bool // open a CDP --remote-debugging-port (default true if unset)

	// Fingerprint and Flags are supplied by the API (P3+). When nil, POC-time
	// pixelscan-green defaults are used. The launcher NEVER sets core here — it
	// is derived from the locally installed core via ResolveCoreDir.
	Fingerprint json.RawMessage
	Flags       json.RawMessage

	Proxy *ProxySpec

	// selenium toggles the legacy ChromeDriver path; left unexported and false —
	// mimic exposes CDP via Puppeteer. Kept for internal experiments only.
	selenium bool
}

// Handle represents a live launched profile. The go-plugin client MUST stay
// alive for the browser's lifetime: chrome depends on the adapter process to
// serve fingerprint-injection config. Stop() tears both down.
type Handle struct {
	ProfileID string
	// CDPPort is chrome's actual --remote-debugging-port (discovered by scan).
	CDPPort int
	// CDPAddress is http://127.0.0.1:<CDPPort>.
	CDPAddress string
	// DriverPort is StartResponse.Value (the adapter's reported ChromeDriver port).
	DriverPort string

	chromePID int32
	adp       browserAdapterV1
	client    *plugin.Client
}

// Launch spawns adapter.exe, initializes it, starts one browser profile, then
// discovers and returns the live CDP endpoint.
func Launch(opts Options) (*Handle, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("mimickit: Options.Name is required")
	}
	if opts.OSType == "" {
		opts.OSType = "windows"
	}

	adapterDir := ResolveCoreDir(opts.AdapterDir)
	adp, client, err := connectAdapter(adapterDir)
	if err != nil {
		return nil, err
	}

	resp, profileID, err := startProfile(adp, adapterDir, opts)
	if err != nil {
		client.Kill()
		return nil, err
	}

	ep, err := findCDPPort(context.Background(), cdpDiscoveryTimeout)
	if err != nil {
		// The adapter is up and the profile started, but no CDP port surfaced.
		// This is exactly the POC gate #2 failure signal — report it, don't hide it.
		_ = adp.Stop(profileID)
		client.Kill()
		return nil, fmt.Errorf("mimickit: profile %s started (driver=%q) but no CDP port found: %w", profileID, resp.Value, err)
	}

	return &Handle{
		ProfileID:  profileID,
		CDPPort:    ep.Port,
		CDPAddress: ep.Address,
		DriverPort: resp.Value,
		chromePID:  ep.PID,
		adp:        adp,
		client:     client,
	}, nil
}

// Stop terminates the browser profile and kills the adapter plugin process.
func (h *Handle) Stop() error {
	err := h.adp.Stop(h.ProfileID)
	h.client.Kill()
	return err
}

// IsActive reports whether the adapter still considers this profile running.
func (h *Handle) IsActive() (bool, error) {
	return h.adp.IsActive(h.ProfileID)
}

// ActivePID returns the chrome PID discovered at launch time.
func (h *Handle) ActivePID() int32 {
	return h.chromePID
}

// startProfile assembles the ReadyProfile payload and issues Plugin.Start.
func startProfile(adp browserAdapterV1, adapterDir string, opts Options) (StartResponse, string, error) {
	profileID := profileIDFromName(opts.Name)
	profileDir := filepath.Join(opts.ProfileDir, opts.Name)
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return StartResponse{}, "", fmt.Errorf("mkdir %s: %w", profileDir, err)
	}

	fp := resolveFingerprint(opts)
	fpRaw, err := assembleReadyProfile(adapterDir, opts, fp)
	if err != nil {
		return StartResponse{}, "", err
	}

	puppeteer := opts.Puppeteer || !opts.selenium // default to CDP path
	req := StartRequest{
		ProfileID:      profileID,
		ProfileDirPath: profileDir,
		Headless:       opts.Headless,
		Puppeteer:      puppeteer,
		Selenium:       opts.selenium,
		FpRaw:          fpRaw,
		Overrides:      buildOverrides(opts, fp),
	}

	resp, err := adp.Start(req)
	if err != nil {
		return StartResponse{}, "", fmt.Errorf("Plugin.Start: %w", err)
	}
	return resp, profileID, nil
}

// resolveFingerprint returns the caller-supplied fingerprint, or the POC-time
// Windows default when none is given. Panics are avoided — a malformed caller
// blob surfaces as an error in assembleReadyProfile instead.
func resolveFingerprint(opts Options) FingerprintData {
	if len(opts.Fingerprint) > 0 {
		var fp FingerprintData
		if err := json.Unmarshal(opts.Fingerprint, &fp); err == nil {
			return fp
		}
	}
	return defaultWindowsFingerprint()
}

// assembleReadyProfile builds the FpRaw payload. Core is ALWAYS set from the
// locally resolved core (validation decision #4); fingerprint + flags come from
// the API, or POC defaults when the caller omits them.
func assembleReadyProfile(adapterDir string, opts Options, fp FingerprintData) ([]byte, error) {
	if len(opts.Fingerprint) > 0 {
		if err := json.Unmarshal(opts.Fingerprint, &fp); err != nil {
			return nil, fmt.Errorf("decode Fingerprint: %w", err)
		}
	}

	var flags ProfileMetaFlags
	if len(opts.Flags) > 0 {
		if err := json.Unmarshal(opts.Flags, &flags); err != nil {
			return nil, fmt.Errorf("decode Flags: %w", err)
		}
	} else {
		flags = defaultGreenFlags()
	}

	major, browserVersion := coreVersions(adapterDir)
	tzFill := true
	rp := ReadyProfile{
		Name:    opts.Name,
		IsLocal: true,
		Core: ReadyProfileCore{
			Browser: ProfileMetaCore{
				BrowserType: "mimic",
				CoreVersion: major,
				OsType:      osTypeFromOSType(opts.OSType),
			},
			BrowserVersion:                browserVersion,
			AutoUpdateCore:                false,
			TimezoneFillBasedOnExternalIP: &tzFill,
		},
		Fingerprint: fp,
		Flags:       flags,
	}

	wrapper := ReadyProfileResponse{
		Data:   &rp,
		Status: ResponseStatus{HTTPCode: 200},
	}
	return json.Marshal(wrapper)
}

// buildOverrides wires the per-launch proxy and geo/timezone overrides from the
// resolved fingerprint. The adapter rejects Start with "timezone overrides is
// not provided" when timezone_masking="mask" and Overrides.Timezone is empty, so
// timezone/publicIP/coordinate are mirrored from the fingerprint here. Proxy is
// the mimic-studio extension required by the plan.
func buildOverrides(opts Options, fp FingerprintData) *LauncherOverrides {
	ov := &LauncherOverrides{}
	if fp.Timezone != nil {
		ov.Timezone = fp.Timezone.Zone
	}
	if fp.Webrtc != nil {
		ov.PublicIP = fp.Webrtc.PublicIP
	}
	if fp.Geolocation != nil {
		ov.Coordinate = Geo{
			Latitude:  fp.Geolocation.Latitude,
			Longitude: fp.Geolocation.Longitude,
			Altitude:  fp.Geolocation.Altitude,
			Accuracy:  fp.Geolocation.Accuracy,
		}
	}
	if opts.Proxy != nil {
		ov.Proxy = &Proxy{
			Type:        opts.Proxy.Type,
			Host:        opts.Proxy.Host,
			Port:        opts.Proxy.Port,
			Username:    opts.Proxy.Username,
			Password:    opts.Proxy.Password,
			SaveTraffic: opts.Proxy.SaveTraffic,
		}
	}
	return ov
}

// connectAdapter spawns adapter.exe via go-plugin, dispenses the browser adapter
// client, and runs Version+Init. Caller owns the returned client and MUST call
// client.Kill() when done (directly or via Handle.Stop()).
func connectAdapter(adapterDir string) (browserAdapterV1, *plugin.Client, error) {
	adapterExe := filepath.Join(adapterDir, adapterExeName)
	if _, err := os.Stat(adapterExe); err != nil {
		return nil, nil, fmt.Errorf("adapter.exe not found at %s: %w", adapterExe, err)
	}

	logger := hclog.New(&hclog.LoggerOptions{Level: hclog.Error, Output: os.Stderr})

	pluginCmd := exec.Command(adapterExe, "--tenant", "mlx")
	pluginCmd.Env = append(os.Environ(),
		"PLUGIN_MIN_PORT=10000",
		"PLUGIN_MAX_PORT=25000",
		"PLUGIN_PROTOCOL_VERSIONS=1",
	)

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: HandshakeConfig,
		VersionedPlugins: map[int]plugin.PluginSet{
			1: PluginMap,
		},
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolNetRPC},
		Cmd:              pluginCmd,
		Logger:           logger,
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("plugin client: %w", err)
	}

	raw, err := rpcClient.Dispense(pluginKey)
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispense: %w", err)
	}
	adp := raw.(browserAdapterV1)

	if _, err := adp.Version(); err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("Version: %w", err)
	}
	if err := adp.Init(adapterDir); err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("Init(%s): %w", adapterDir, err)
	}

	return adp, client, nil
}

// osTypeFromOSType maps the caller's OSType to the adapter's os_type enum.
func osTypeFromOSType(osType string) string {
	switch osType {
	case "macos", "MacIntel", "MacARM":
		return "MacIntel"
	case "linux", "Linux":
		return "Linux"
	default:
		return "Win32"
	}
}

// profileIDFromName returns a deterministic UUID-format string. adapter.exe
// validates profileId as a UUID; a non-UUID format triggers "wrong UUID".
func profileIDFromName(name string) string {
	s := nameToSeed(name)
	a := uint32(s >> 32)
	b := uint16(s >> 16)
	c := uint16(s&0xFFFF) | 0x4000    // version 4
	d := uint16(s>>8&0x3FFF) | 0x8000 // variant bits
	e := uint64(s) & 0xFFFFFFFFFFFF
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", a, b, c, d, e)
}

func nameToSeed(name string) int64 {
	h := int64(0)
	for _, c := range name {
		h = h*31 + int64(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
