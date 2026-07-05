package standalone

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// synthDLL builds a small buffer that embeds the gate signature between filler
// bytes so patch/restore can be exercised without a 200MB real chrome.dll.
func synthDLL(withSig bool) []byte {
	prefix := bytes.Repeat([]byte{0xCC}, 16)
	suffix := bytes.Repeat([]byte{0xEE}, 16)
	mid := gateSig
	if !withSig {
		mid = bytes.Repeat([]byte{0x00}, len(gateSig))
	}
	out := append([]byte{}, prefix...)
	out = append(out, mid...)
	return append(out, suffix...)
}

func writeDLL(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "chrome.dll")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write synth dll: %v", err)
	}
	return p
}

func TestPatchCore_NopsGate(t *testing.T) {
	orig := synthDLL(true)
	p := writeDLL(t, orig)
	if err := PatchCore(p); err != nil {
		t.Fatalf("PatchCore: %v", err)
	}
	got, _ := os.ReadFile(p)
	idx := bytes.Index(orig, gateSig)
	je := idx + jeOffset
	if got[je] != 0x90 || got[je+1] != 0x90 {
		t.Errorf("gate bytes = %02x %02x, want 90 90", got[je], got[je+1])
	}
	// Every byte except the two gate bytes must be untouched.
	for i := range orig {
		if i == je || i == je+1 {
			continue
		}
		if got[i] != orig[i] {
			t.Errorf("byte %d changed: %02x -> %02x", i, orig[i], got[i])
		}
	}
}

func TestPatchCore_Idempotent(t *testing.T) {
	p := writeDLL(t, synthDLL(true))
	if err := PatchCore(p); err != nil {
		t.Fatalf("first patch: %v", err)
	}
	after1, _ := os.ReadFile(p)
	if err := PatchCore(p); err != nil {
		t.Fatalf("second patch: %v", err)
	}
	after2, _ := os.ReadFile(p)
	if !bytes.Equal(after1, after2) {
		t.Error("second patch changed content; want no-op")
	}
}

func TestPatchCore_BackupOnce(t *testing.T) {
	orig := synthDLL(true)
	p := writeDLL(t, orig)
	bak := p + ".bak"
	if err := PatchCore(p); err != nil {
		t.Fatalf("patch: %v", err)
	}
	b1, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if !bytes.Equal(b1, orig) {
		t.Error("backup != original")
	}
	// A second patch must not overwrite the backup with patched content.
	if err := PatchCore(p); err != nil {
		t.Fatalf("second patch: %v", err)
	}
	b2, _ := os.ReadFile(bak)
	if !bytes.Equal(b2, orig) {
		t.Error("backup overwritten on second patch")
	}
}

func TestPatchCore_SignatureNotFound(t *testing.T) {
	orig := synthDLL(false)
	p := writeDLL(t, orig)
	err := PatchCore(p)
	if err == nil {
		t.Fatal("want error for missing signature, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("signature not found")) {
		t.Errorf("error = %q, want to mention 'signature not found'", err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, orig) {
		t.Error("file modified despite missing signature (not fail-safe)")
	}
	if _, statErr := os.Stat(p + ".bak"); statErr == nil {
		t.Error("backup written despite missing signature")
	}
}

func TestRestoreCore_RoundTrip(t *testing.T) {
	orig := synthDLL(true)
	p := writeDLL(t, orig)
	if err := PatchCore(p); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := RestoreCore(p); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, orig) {
		t.Error("restored content != original")
	}
}

func TestRestoreCore_MissingBackup(t *testing.T) {
	p := writeDLL(t, synthDLL(true))
	err := RestoreCore(p)
	if err == nil {
		t.Fatal("want error for missing backup, got nil")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("no backup")) {
		t.Errorf("error = %q, want to mention 'no backup'", err)
	}
}
