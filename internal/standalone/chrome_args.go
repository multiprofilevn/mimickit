package standalone

import (
	"net"
	"strconv"
	"strings"
)

// BuildChromeArgs constructs the chrome command line: the managed mimic flags
// (unless ManagedFlags is explicitly false) followed by the caller's arbitrary
// ChromeArgs. User args come LAST so they can override or extend anything.
func BuildChromeArgs(c Config) []string {
	var a []string
	if c.ManagedFlags == nil || *c.ManagedFlags {
		a = append(a,
			"--disable-quic",
			"--session_name="+c.SessionName,
			"--user-agent="+c.UserAgent,
			"--platform="+c.Platform,
			"--platform_version="+c.PlatformVersion,
			"--accept_language="+c.AcceptLanguage,
			"--client-session-id="+c.SessionID,
			"--width="+strconv.Itoa(c.Width),
			"--primary_language="+firstLang(c.Languages),
			"--languages="+c.Languages,
			"--port_whitelist=["+strconv.Itoa(c.ClientPort)+","+intListInner(c.PortWhitelist)+"]",
			"--client-port="+strconv.Itoa(c.ClientPort),
			"--disable-blink-features=,AutomationControlled",
			"--ch-add-brand-google-chrome",
			"--ozone-override-screen-size="+strconv.Itoa(c.Width)+","+strconv.Itoa(c.Height),
			"--user-data-dir="+c.UserDataDir,
			"--remote-debugging-port="+strconv.Itoa(c.RemoteDebuggingPort),
			"--height="+strconv.Itoa(c.Height),
		)
	} else {
		// Minimal set still required for fp injection + CDP even when unmanaged.
		a = append(a,
			"--client-port="+strconv.Itoa(c.ClientPort),
			"--client-session-id="+c.SessionID,
			"--user-data-dir="+c.UserDataDir,
			"--remote-debugging-port="+strconv.Itoa(c.RemoteDebuggingPort),
		)
	}
	// Extensions load in both managed and minimal modes (they matter regardless of
	// the fingerprint flag set). Placed before user args so ChromeArgs can still
	// override with a later --load-extension if desired.
	if len(c.LoadExtensions) > 0 {
		a = append(a, "--load-extension="+strings.Join(c.LoadExtensions, ","))
	}
	return append(a, c.ChromeArgs...)
}

func intListString(xs []int) string { return "[" + intListInner(xs) + "]" }

func intListInner(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}

func firstLang(langs string) string {
	if i := strings.IndexByte(langs, ','); i >= 0 {
		return langs[:i]
	}
	return langs
}

// FreePort asks the OS for an unused loopback TCP port.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
