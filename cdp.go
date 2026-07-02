package mimickit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

// cdpTarget is a minimal CDP target descriptor from the /json endpoint.
type cdpTarget struct {
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// cdpEndpoint is the result of locating a live CDP debug port.
type cdpEndpoint struct {
	Port       int    // Chrome's --remote-debugging-port
	Address    string // http://127.0.0.1:<port>
	PID        int32  // owning chrome.exe PID
	WSDebugURL string // webSocketDebuggerUrl of the first page target (may be empty)
}

// findCDPPort scans running chrome processes for a --remote-debugging-port flag,
// then confirms the port answers /json before returning. Retries with backoff
// until the deadline because the adapter opens the debug port a beat after Start
// returns. Returns an error if no live CDP endpoint is found in time.
func findCDPPort(ctx context.Context, timeout time.Duration) (*cdpEndpoint, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		port, pid := scanChromeDebugPort()
		if port > 0 {
			addr := fmt.Sprintf("http://127.0.0.1:%d", port)
			if ws, err := probeCDP(addr); err == nil {
				return &cdpEndpoint{Port: port, Address: addr, PID: pid, WSDebugURL: ws}, nil
			} else {
				lastErr = err
			}
		} else {
			lastErr = fmt.Errorf("no chrome process with --remote-debugging-port")
		}
		time.Sleep(150 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return nil, fmt.Errorf("findCDPPort: %w", lastErr)
}

// scanChromeDebugPort walks processes, finds a chrome binary launched with
// --remote-debugging-port=<n> (skipping crashpad helpers), and returns the port
// plus the owning PID. Returns (0, 0) when none is found.
func scanChromeDebugPort() (int, int32) {
	procs, err := process.Processes()
	if err != nil {
		return 0, 0
	}
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || !isChromeProcessName(name) {
			continue
		}
		cmdline, err := p.Cmdline()
		if err != nil || cmdline == "" {
			continue
		}
		if strings.Contains(cmdline, "crashpad") {
			continue
		}
		if port := parseDebugPort(cmdline); port > 0 {
			return port, p.Pid
		}
	}
	return 0, 0
}

func isChromeProcessName(name string) bool {
	n := strings.ToLower(name)
	return n == "chrome.exe" || n == "chrome"
}

// parseDebugPort extracts the numeric value of --remote-debugging-port from a
// process command line. Returns 0 when absent or 0-valued.
func parseDebugPort(cmdline string) int {
	const flag = "--remote-debugging-port="
	for _, part := range strings.Fields(cmdline) {
		if !strings.HasPrefix(part, flag) {
			continue
		}
		val := strings.TrimPrefix(part, flag)
		var port int
		if _, err := fmt.Sscanf(val, "%d", &port); err == nil && port > 0 {
			return port
		}
	}
	return 0
}

// probeCDP hits <addr>/json and returns the webSocketDebuggerUrl of the first
// page target. An empty page target list is still a valid endpoint (returns "").
func probeCDP(addr string) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(addr + "/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var targets []cdpTarget
	if err := json.Unmarshal(body, &targets); err != nil {
		return "", fmt.Errorf("decode /json: %w", err)
	}
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", nil
}
