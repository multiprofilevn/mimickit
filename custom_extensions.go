package mimickit

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CustomExtensionsDir returns the folder the mimic adapter scans for unpacked
// extensions. Reverse-engineered from adapter.exe
// (adapterMimic.(*MimicAdapter).addExtensions):
//
//	dir := path.Join(datadir.GetDataDir(), "custom_extensions", "mimic")
//	for _, e := range os.ReadDir(dir) { paths = append(paths, path.Join(dir, e)) }
//	chromeFlags["load-extension"] = strings.Join(paths, ",")
//
// where datadir.GetDataDir() == filepath.Join(<home>, "mlx"). So every DIRECT
// subdirectory of <home>/mlx/custom_extensions/mimic is passed to chrome via
// --load-extension at launch. There is no per-profile variant — the folder is
// global and shared by every mimic profile. Returns "" if the home dir can't be
// resolved (mirrors ResolveCoreDir's fail-soft behavior).
func CustomExtensionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "mlx", "custom_extensions", "mimic")
}

// InstallCustomExtension copies an unpacked extension at srcDir into the mimic
// custom-extensions folder under a stable subdir <name>, so the adapter loads it
// on the next launch. The target is replaced wholesale (RemoveAll + copy) so a
// version bump doesn't leave stale files behind. name must be a single path
// segment (no separators) — it becomes one --load-extension entry.
func InstallCustomExtension(name, srcDir string) error {
	if name == "" || srcDir == "" {
		return fmt.Errorf("mimickit: InstallCustomExtension requires name and srcDir")
	}
	base := CustomExtensionsDir()
	if base == "" {
		return fmt.Errorf("mimickit: custom extensions dir unresolved (no home dir)")
	}
	target := filepath.Join(base, name)
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("clear %s: %w", target, err)
	}
	if err := copyDirRecursive(srcDir, target); err != nil {
		return fmt.Errorf("copy extension %s: %w", name, err)
	}
	return nil
}

// RemoveCustomExtension deletes the <name> subdir from the mimic custom-extensions
// folder (idempotent — a missing dir is not an error). Used to unload a builtin
// extension whose toggle was turned off, without touching any other subdir.
func RemoveCustomExtension(name string) error {
	if name == "" {
		return fmt.Errorf("mimickit: RemoveCustomExtension requires name")
	}
	base := CustomExtensionsDir()
	if base == "" {
		return nil
	}
	return os.RemoveAll(filepath.Join(base, name))
}

// copyDirRecursive copies the file tree at src into dst, creating dst and any
// parents. Symlinks are followed as regular files (extension builds are plain
// files). Mirrors pkg/resource.copyTree in the app; kept local so mimickit stays
// dependency-free.
func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
