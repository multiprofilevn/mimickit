package standalone

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// rawGet opens a raw TCP connection and sends a minimal HTTP/1.0 request,
// returning the full response bytes — mirroring mimic chrome's lenient fetcher.
func rawGet(t *testing.T, port int) []byte {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("GET /mimic/start/x HTTP/1.0\r\n\r\n"))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var out bytes.Buffer
	_, _ = out.ReadFrom(conn)
	return out.Bytes()
}

// bodyOf splits an HTTP response into its body (after the blank line).
func bodyOf(raw []byte) string {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return string(raw[i+4:])
	}
	return ""
}

func TestFPServer_ServesAESPayload(t *testing.T) {
	params := []byte(`{"platform":"Win32","gl_vendor":"Google Inc. (NVIDIA)"}`)
	s, err := NewFPServer(params)
	if err != nil {
		t.Fatalf("NewFPServer: %v", err)
	}
	port, err := s.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	raw := rawGet(t, port)
	enc, err := base64.StdEncoding.DecodeString(strings.TrimSpace(bodyOf(raw)))
	if err != nil {
		t.Fatalf("base64 decode body: %v", err)
	}
	dec, err := aesDecrypt(enc)
	if err != nil {
		t.Fatalf("aes decrypt: %v", err)
	}
	if !bytes.Equal(dec, params) {
		t.Errorf("decrypted = %s, want %s", dec, params)
	}
}

func TestFPServer_RawHTTP10(t *testing.T) {
	s, _ := NewFPServer([]byte(`{}`))
	port, err := s.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer s.Close()

	raw := rawGet(t, port)
	line, _ := bufio.NewReader(bytes.NewReader(raw)).ReadString('\n')
	if !strings.HasPrefix(line, "HTTP/1.0 200") {
		t.Errorf("status line = %q, want HTTP/1.0 200 prefix", strings.TrimSpace(line))
	}
}

func TestFPServer_CloseNoLeak(t *testing.T) {
	s, _ := NewFPServer([]byte(`{}`))
	port, err := s.Start(0)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// A connection after Close must fail to establish.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err == nil {
		conn.Close()
		t.Error("dial succeeded after Close; listener leaked")
	}
}

func TestBuildFingerprintPayload_RoundTrip(t *testing.T) {
	cfg := Config{
		Platform:      "Win32",
		GLVendor:      "Google Inc. (NVIDIA)",
		Width:         1920,
		Height:        1080,
		HWConcurrency: 8,
		Timezone:      "America/New_York",
		PortWhitelist: []int{80, 443},
		SessionFlags:  map[string]string{"screen_masking": "mask"},
	}
	plain, err := BuildFingerprintPayload(cfg)
	if err != nil {
		t.Fatalf("BuildFingerprintPayload: %v", err)
	}
	if !bytes.Contains(plain, []byte(`"platform":"Win32"`)) {
		t.Errorf("payload missing platform: %s", plain)
	}
	if !bytes.Contains(plain, []byte(`"port_whitelist":"[80,443]"`)) {
		t.Errorf("payload port_whitelist malformed: %s", plain)
	}
}
