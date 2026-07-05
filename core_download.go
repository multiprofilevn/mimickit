package mimickit

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tuwibu/mimickit/internal/standalone"
)

// patchMarkerName records which core version PatchCore was last applied to. It
// lets the launcher skip re-reading the ~100-200MB DLL on every launch: a cheap
// marker read decides whether a re-patch is needed (e.g. after chrome
// auto-update wiped the patch).
const patchMarkerName = ".mimickit-patched"

// CoreOpts configures where EnsureCore fetches the mimic core from. Secrets (R2
// credentials) are NOT held here — BaseURL is expected to be a signed/public
// URL supplied by the consumer's app config. Version pins the signature-coupled
// build (149.2 today); Checksum, when set, guards download integrity.
type CoreOpts struct {
	BaseURL  string // e.g. https://r2.example.com/cores
	Version  string // e.g. "149.2"
	Checksum string // hex sha256 of the zip; "" => skip verification

	// Test seams; nil => real implementations.
	download func(url string) ([]byte, error)
	patch    func(dll string) error
}

// EnsureCore makes coreDir hold a patched mimic core at opts.Version, downloading
// from R2 when absent. It is idempotent: a valid, correctly-patched core is a
// no-op. Download → checksum → extract is atomic (staged in a temp dir and
// renamed) so a failed download never leaves a half-written core. Returns the
// resolved core version.
func EnsureCore(coreDir string, opts CoreOpts) (string, error) {
	if opts.Version == "" {
		return "", fmt.Errorf("EnsureCore: opts.Version is required")
	}
	patch := opts.patch
	if patch == nil {
		patch = standalone.PatchCore
	}
	chromeDLL := filepath.Join(coreDir, "chrome.dll")

	if fileExists(chromeDLL) {
		if !needsRepatch(coreDir, opts.Version) {
			return opts.Version, nil
		}
		// Core present but patch stale (version changed) — re-patch in place.
		if err := patch(chromeDLL); err != nil {
			return "", fmt.Errorf("re-patch %s: %w", chromeDLL, err)
		}
		if err := writePatchMarker(coreDir, opts.Version); err != nil {
			return "", err
		}
		return opts.Version, nil
	}

	// Core missing — download + extract + patch atomically.
	if opts.BaseURL == "" {
		return "", fmt.Errorf("EnsureCore: core missing and no BaseURL to download from")
	}
	download := opts.download
	if download == nil {
		download = httpDownload
	}
	zipBytes, err := download(coreURL(opts.BaseURL, opts.Version))
	if err != nil {
		return "", fmt.Errorf("download core: %w", err)
	}
	if opts.Checksum != "" {
		sum := sha256.Sum256(zipBytes)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, opts.Checksum) {
			return "", fmt.Errorf("core checksum mismatch: got %s want %s", got, opts.Checksum)
		}
	}

	tmp := coreDir + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := extractZip(zipBytes, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("extract core: %w", err)
	}
	tmpDLL := filepath.Join(tmp, "chrome.dll")
	if fileExists(tmpDLL) {
		if err := patch(tmpDLL); err != nil {
			_ = os.RemoveAll(tmp)
			return "", fmt.Errorf("patch core: %w", err)
		}
	}
	if err := writePatchMarker(tmp, opts.Version); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	_ = os.RemoveAll(coreDir)
	if err := os.Rename(tmp, coreDir); err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("promote core: %w", err)
	}
	return opts.Version, nil
}

// needsRepatch reports whether coreDir must be re-patched: true when the patch
// marker is missing or records a different version than wanted. The read is
// cheap (a tiny marker file), never the full DLL.
func needsRepatch(coreDir, version string) bool {
	b, err := os.ReadFile(filepath.Join(coreDir, patchMarkerName))
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(b)) != version
}

func writePatchMarker(coreDir, version string) error {
	return os.WriteFile(filepath.Join(coreDir, patchMarkerName), []byte(version), 0o644)
}

func coreURL(baseURL, version string) string {
	return strings.TrimRight(baseURL, "/") + "/mimic_" + version + ".zip"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func httpDownload(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractZip unpacks a zip archive into dest, guarding against path traversal
// (zip-slip). Directories are created as needed.
func extractZip(data []byte, dest string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		target := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != filepath.Clean(dest) {
			return fmt.Errorf("illegal zip path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}
