package mimickit

import (
	"os"
	"path/filepath"
	"testing"
)

// redirectHome points os.UserHomeDir at a temp dir for the duration of the test.
// On Windows os.UserHomeDir reads %USERPROFILE%; elsewhere $HOME.
func redirectHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("HOME", tmp)
	return tmp
}

func TestCustomExtensionsDir(t *testing.T) {
	home := redirectHome(t)
	want := filepath.Join(home, "mlx", "custom_extensions", "mimic")
	if got := CustomExtensionsDir(); got != want {
		t.Fatalf("CustomExtensionsDir() = %q, want %q", got, want)
	}
}

func TestInstallAndRemoveCustomExtension(t *testing.T) {
	redirectHome(t)

	// Build a fake unpacked extension: manifest + a nested asset.
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "manifest.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "js"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "js", "bg.js"), []byte("//bg"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallCustomExtension("mouse-helper", src); err != nil {
		t.Fatalf("InstallCustomExtension: %v", err)
	}

	target := filepath.Join(CustomExtensionsDir(), "mouse-helper")
	for _, rel := range []string{"manifest.json", filepath.Join("js", "bg.js")} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Fatalf("expected copied file %s: %v", rel, err)
		}
	}

	// Re-install replaces wholesale: a stale file from a prior version is gone.
	stale := filepath.Join(target, "old.js")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallCustomExtension("mouse-helper", src); err != nil {
		t.Fatalf("re-install: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale file survived wholesale replace: err=%v", err)
	}

	if err := RemoveCustomExtension("mouse-helper"); err != nil {
		t.Fatalf("RemoveCustomExtension: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target dir survived removal: err=%v", err)
	}

	// Remove is idempotent — a second call on a missing dir is not an error.
	if err := RemoveCustomExtension("mouse-helper"); err != nil {
		t.Fatalf("idempotent RemoveCustomExtension: %v", err)
	}
}
