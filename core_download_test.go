package mimickit

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// gateSigBytes duplicates internal/standalone's gate signature so the test can
// build a synthetic chrome.dll the real PatchCore will recognize.
var gateSigBytes = []byte{0xB3, 0x01, 0x48, 0x85, 0xC0, 0x74, 0x39, 0x48, 0x8B, 0x84, 0x24}

func synthDLLWithSig() []byte {
	return append(append(bytes.Repeat([]byte{0xCC}, 8), gateSigBytes...), bytes.Repeat([]byte{0xEE}, 8)...)
}

func buildCoreZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("chrome.dll")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write(synthDLLWithSig()); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestEnsureCore_NoopWhenValid(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mimic_149.2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "chrome.dll"), synthDLLWithSig(), 0o644)
	writePatchMarker(dir, "149.2")

	opts := CoreOpts{
		Version: "149.2",
		download: func(string) ([]byte, error) {
			t.Fatal("download called for a valid core")
			return nil, nil
		},
	}
	v, err := EnsureCore(dir, opts)
	if err != nil {
		t.Fatalf("EnsureCore: %v", err)
	}
	if v != "149.2" {
		t.Errorf("version = %q, want 149.2", v)
	}
}

func TestEnsureCore_DownloadsWhenMissing(t *testing.T) {
	zipBytes := buildCoreZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer srv.Close()

	sum := sha256.Sum256(zipBytes)
	dir := filepath.Join(t.TempDir(), "mimic_149.2")
	opts := CoreOpts{BaseURL: srv.URL, Version: "149.2", Checksum: hex.EncodeToString(sum[:])}

	v, err := EnsureCore(dir, opts)
	if err != nil {
		t.Fatalf("EnsureCore: %v", err)
	}
	if v != "149.2" {
		t.Errorf("version = %q, want 149.2", v)
	}
	got, err := os.ReadFile(filepath.Join(dir, "chrome.dll"))
	if err != nil {
		t.Fatalf("chrome.dll not extracted: %v", err)
	}
	je := bytes.Index(synthDLLWithSig(), gateSigBytes) + 5
	if got[je] != 0x90 || got[je+1] != 0x90 {
		t.Error("extracted chrome.dll was not patched")
	}
	if needsRepatch(dir, "149.2") {
		t.Error("marker not written after download")
	}
}

func TestEnsureCore_ChecksumMismatch(t *testing.T) {
	zipBytes := buildCoreZip(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(zipBytes)
	}))
	defer srv.Close()

	dir := filepath.Join(t.TempDir(), "mimic_149.2")
	opts := CoreOpts{BaseURL: srv.URL, Version: "149.2", Checksum: "deadbeef"}

	if _, err := EnsureCore(dir, opts); err == nil {
		t.Fatal("want checksum mismatch error, got nil")
	}
	if fileExists(filepath.Join(dir, "chrome.dll")) {
		t.Error("partial core written despite checksum mismatch (not atomic)")
	}
}

func TestRepatchGuard_VersionChanged(t *testing.T) {
	dir := t.TempDir()
	writePatchMarker(dir, "148.0")
	if !needsRepatch(dir, "149.2") {
		t.Error("version changed but needsRepatch = false")
	}
}

func TestRepatchGuard_SameVersion(t *testing.T) {
	dir := t.TempDir()
	writePatchMarker(dir, "149.2")
	if needsRepatch(dir, "149.2") {
		t.Error("same version but needsRepatch = true")
	}
}
