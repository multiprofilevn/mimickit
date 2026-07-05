package standalone

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

// client-port fingerprint encryption. These AES-128-CBC key/iv are baked into
// mimic chrome.dll (RE'd from adapter.exe, mimic 149.2); serving the same
// encryption makes chrome apply masking with no adapter/cloud dependency. They
// are version-coupled to the 149.2 core the R2 download pins.
var (
	fpKey = []byte("283DEB9E12B137N0")
	fpIV  = []byte("9Ak48190aAfnbP02")
)

// FPServer is a raw TCP HTTP/1.0 responder that hands the AES-encrypted
// fingerprint to mimic chrome's client-port fetcher. It must be HTTP/1.0 and
// byte-lenient: chrome's fetcher sends a minimal request and rejects Go's
// stricter net/http HTTP/1.1 replies. Any request receives the fingerprint.
type FPServer struct {
	ln      net.Listener
	payload string // base64(AES-128-CBC(params))
	resp    string // full HTTP/1.0 wire response
}

// NewFPServer encrypts params (plaintext JSON) into the client-port payload and
// prepares the wire response. It does NOT bind yet — call Start.
func NewFPServer(params []byte) (*FPServer, error) {
	enc, err := aesEncrypt(params)
	if err != nil {
		return nil, err
	}
	payload := base64.StdEncoding.EncodeToString(enc)
	resp := "HTTP/1.0 200 OK\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"Content-Length: " + strconv.Itoa(len(payload)) + "\r\n" +
		"\r\n" + payload
	return &FPServer{payload: payload, resp: resp}, nil
}

// Start binds a loopback listener on the given port (0 => auto-allocate) and
// serves the fingerprint until Close. Returns the bound port.
func (s *FPServer) Start(port int) (int, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return 0, fmt.Errorf("client-port listen: %w", err)
	}
	s.ln = ln
	go s.serve()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// Port returns the bound port, or 0 before Start.
func (s *FPServer) Port() int {
	if s.ln == nil {
		return 0
	}
	return s.ln.Addr().(*net.TCPAddr).Port
}

func (s *FPServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
			buf := make([]byte, 2048)
			_, _ = c.Read(buf) // request content is irrelevant; any request served
			_, _ = c.Write([]byte(s.resp))
		}(conn)
	}
}

// Close releases the listener, stopping the accept loop.
func (s *FPServer) Close() error {
	if s.ln == nil {
		return nil
	}
	return s.ln.Close()
}

// BuildFingerprintPayload assembles the flat client-port params from Config. The
// param keys are the exact names mimic chrome.dll reads after decryption.
func BuildFingerprintPayload(c Config) ([]byte, error) {
	rtc := "1"
	if c.RTCBlock != nil && !*c.RTCBlock {
		rtc = "0"
	}
	flagsJSON, err := json.Marshal(c.SessionFlags)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"automation_type": "2",
		"platform":        c.Platform,
		"gl_vendor":       c.GLVendor,
		"gl_renderer":     c.GLRenderer,
		"width":           strconv.Itoa(c.Width),
		"height":          strconv.Itoa(c.Height),
		"hw_concurrency":  strconv.Itoa(c.HWConcurrency),
		"timezone":        c.Timezone,
		"port_whitelist":  intListString(c.PortWhitelist),
		"rtc_block":       rtc,
		"session-flags":   string(flagsJSON),
		"tm":              "0",
		"token":           strconv.FormatInt(time.Now().Unix(), 10),
	}
	if c.Geo != nil {
		params["geo_type"] = strconv.Itoa(c.Geo.Type)
		params["geo_acc"] = fmt.Sprintf("%.6f", c.Geo.Acc)
		params["geo_lat"] = fmt.Sprintf("%.6f", c.Geo.Lat)
		params["geo_long"] = fmt.Sprintf("%.6f", c.Geo.Long)
	}
	for k, v := range c.ExtraFingerprint {
		params[k] = v
	}
	return json.Marshal(params)
}

func aesEncrypt(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(fpKey)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(plain, block.BlockSize())
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, fpIV).CryptBlocks(out, padded)
	return out, nil
}

// aesDecrypt is the inverse of aesEncrypt, used by tests to assert the round trip.
func aesDecrypt(enc []byte) ([]byte, error) {
	block, err := aes.NewCipher(fpKey)
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 || len(enc)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("bad ciphertext length %d", len(enc))
	}
	out := make([]byte, len(enc))
	cipher.NewCBCDecrypter(block, fpIV).CryptBlocks(out, enc)
	return pkcs7Unpad(out)
}

func pkcs7Pad(b []byte, size int) []byte {
	pad := size - len(b)%size
	return append(b, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > len(b) {
		return nil, fmt.Errorf("bad padding %d", pad)
	}
	return b[:len(b)-pad], nil
}
