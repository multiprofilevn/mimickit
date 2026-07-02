package mimickit

// POC defaults. Once the API (P3) supplies fingerprint + flags, these are only a
// fallback. The flag combination below is the exact pixelscan-green ("No masking
// detected") config bisected against genuine Multilogin — do NOT change a value
// without re-verifying against pixelscan.net.

func strPtr(s string) *string { return &s }

// defaultGreenFlags returns the verified pixelscan-green masking configuration.
func defaultGreenFlags() ProfileMetaFlags {
	return ProfileMetaFlags{
		// "natural" — masking audio triggers "wrong audio params" / "invalid audio flag".
		AudioMasking: "natural",
		// canvas_noise="mask" is invalid ("applying graphics_noise"); omit for default.
		CanvasNoise: nil,
		// "mask" on fonts stalls pixelscan at "Collecting Data" — keep natural.
		FontsMasking:       "natural",
		GeolocationMasking: "mask",
		GeolocationPopup:   "prompt",
		GraphicsMasking:    "mask",
		// "natural" leaves the real hardware canvas so toDataURL works immediately
		// (masking noise's async C++ hook isn't ready at early page load).
		GraphicsNoise:       "natural",
		LocalizationMasking: "mask",
		MediaDevicesMasking: "natural",
		NavigatorMasking:    "mask",
		PortsMasking:        "mask",
		ProxyMasking:        "disabled",
		QuicMode:            strPtr("disabled"),
		ScreenMasking:       "mask",
		StartupBehavior:     strPtr("recover"),
		TimezoneMasking:     "mask",
		WebrtcMasking:       "disabled",
	}
}

// defaultWindowsFingerprint returns a minimal, self-consistent Windows Chrome 149
// fingerprint sufficient to launch a profile during the POC. Screen size uses the
// real primary display so mimic's window-bounds lock matches the monitor.
func defaultWindowsFingerprint() FingerprintData {
	w, h := realScreenSize()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36"
	fonts := []string{
		"Arial", "Arial Black", "Calibri", "Cambria", "Comic Sans MS",
		"Consolas", "Courier New", "Georgia", "Segoe UI", "Tahoma",
		"Times New Roman", "Trebuchet MS", "Verdana",
	}
	ports := []int{}
	return FingerprintData{
		Navigator: &FPNavigator{
			HardwareConcurrency: 8,
			Platform:            "Win32",
			UserAgent:           ua,
			PlatformVersion:     strPtr("15.0.0"),
		},
		Screen: &FPScreen{Width: w, Height: h, PixelRatio: 1.0},
		Graphic: &FPGraphic{
			Vendor:   "Google Inc. (NVIDIA)",
			Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3060 Direct3D11 vs_5_0 ps_5_0, D3D11)",
			VendorID: strPtr("0x10de"),
			DeviceID: strPtr("0x2504"),
		},
		Localization: &FPLocalization{
			AcceptLanguages: "en-US,en;q=0.9",
			Languages:       "en-US,en",
			Locale:          "en-US",
		},
		MediaDevices: &FPMediaDevices{AudioInputs: 1, AudioOutputs: 1, VideoInputs: 1},
		// geolocation_masking="mask" requires a coordinate; use one consistent with
		// the default timezone (New York).
		Geolocation: &FPGeolocation{Latitude: 40.7128, Longitude: -74.0060, Altitude: 0, Accuracy: 100},
		Timezone:    &FPTimezone{Zone: "America/New_York"},
		Webrtc:      &FPWebrtc{PublicIP: ""},
		Fonts:       &fonts,
		Ports:       &ports,
	}
}
