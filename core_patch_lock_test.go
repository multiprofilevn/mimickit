package mimickit

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubPatchCore replaces the real DLL patcher for the duration of a test and
// returns a counter of how many times it ran. The real one rewrites hundreds of
// megabytes; what these tests care about is how OFTEN it is called.
func stubPatchCore(t *testing.T, fn func(dll string) error) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	prev := patchCore
	patchCore = func(dll string) error {
		calls.Add(1)
		if fn != nil {
			return fn(dll)
		}
		return nil
	}
	t.Cleanup(func() { patchCore = prev })
	return &calls
}

// newUnpatchedCore builds a core directory holding a stand-in chrome.dll and no
// patch marker, i.e. the cold state where the race lives.
func newUnpatchedCore(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chrome.dll"), []byte("stand-in"), 0o644); err != nil {
		t.Fatalf("write chrome.dll: %v", err)
	}
	return dir
}

// TestPatchLocalCore_ConcurrentPatchesOnlyOnce is the regression test for the
// corruption bug: many profiles launching at once against one cold core.
//
// Two details make it actually detect the bug rather than pass by luck. A start
// barrier releases every goroutine at the same moment, so they all reach the
// "is it patched yet?" check before any of them has written the marker. And the
// stubbed patch is slow enough to still be running when the others arrive —
// mimicking the real one, which rewrites hundreds of megabytes. Without both,
// the first goroutine finishes before the rest start and an unlocked
// implementation passes.
func TestPatchLocalCore_ConcurrentPatchesOnlyOnce(t *testing.T) {
	dir := newUnpatchedCore(t, "mimic_149.2")
	calls := stubPatchCore(t, func(string) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	const goroutines = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = patchLocalCore(dir)
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: patchLocalCore = %v, want nil", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("patchCore ran %d times across %d launches, want exactly 1", got, goroutines)
	}
}

func TestPatchLocalCore_DifferentDirsDoNotBlock(t *testing.T) {
	a := newUnpatchedCore(t, "mimic_148.0")
	b := newUnpatchedCore(t, "mimic_149.2")

	// Each patch holds its lock while the other is trying to take one. With a
	// per-directory lock both finish; with a global lock they serialise and the
	// elapsed time doubles, but the real failure this guards is a deadlock.
	release := make(chan struct{})
	calls := stubPatchCore(t, func(string) error {
		<-release
		return nil
	})

	done := make(chan error, 2)
	go func() { done <- patchLocalCore(a) }()
	go func() { done <- patchLocalCore(b) }()

	time.Sleep(100 * time.Millisecond)
	if got := calls.Load(); got != 2 {
		t.Errorf("patchCore entered %d times, want 2 — the two dirs are blocking each other", got)
	}
	close(release)

	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("patchLocalCore = %v, want nil", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("patchLocalCore did not return — deadlock")
		}
	}
}

func TestPatchLocalCore_AlreadyPatchedIsNoOp(t *testing.T) {
	dir := newUnpatchedCore(t, "mimic_149.2")
	if err := writePatchMarker(dir, "149.2"); err != nil {
		t.Fatalf("writePatchMarker: %v", err)
	}
	calls := stubPatchCore(t, nil)

	if err := patchLocalCore(dir); err != nil {
		t.Fatalf("patchLocalCore = %v, want nil", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("patchCore ran %d times on an already-patched core, want 0", got)
	}
}

func TestPatchLocalCore_MissingDLLIsNoOp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mimic_149.2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	calls := stubPatchCore(t, nil)

	if err := patchLocalCore(dir); err != nil {
		t.Fatalf("patchLocalCore = %v, want nil", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("patchCore ran %d times without chrome.dll, want 0", got)
	}
	if _, err := os.Stat(filepath.Join(dir, patchLockName)); err == nil {
		t.Error("a lock file was created for a core that has nothing to patch")
	}
}

// TestPatchLock_ReleasedOnPatchError pins the worst outcome of a missed release:
// one failed patch would lock the core out for every later launch.
func TestPatchLock_ReleasedOnPatchError(t *testing.T) {
	dir := newUnpatchedCore(t, "mimic_149.2")
	wantErr := errors.New("gate signature not found")
	stubPatchCore(t, func(string) error { return wantErr })

	if err := patchLocalCore(dir); !errors.Is(err, wantErr) {
		t.Fatalf("patchLocalCore = %v, want %v", err, wantErr)
	}
	if _, err := os.Stat(filepath.Join(dir, patchLockName)); err == nil {
		t.Error("lock file survived a failed patch — the core is now wedged")
	}
}

func TestPatchLock_StaleLockIsReclaimed(t *testing.T) {
	dir := newUnpatchedCore(t, "mimic_149.2")
	lock := filepath.Join(dir, patchLockName)
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	old := time.Now().Add(-2 * patchLockStaleAfter)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	start := time.Now()
	release, err := acquirePatchLock(dir)
	if err != nil {
		t.Fatalf("acquirePatchLock over a stale lock = %v, want success", err)
	}
	release()

	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("waited %s for a stale lock, want it reclaimed immediately", elapsed)
	}
}

// TestPatchLock_ConcurrentAcquireOnlyOneWins asserts the lock is exclusive at
// every instant, which is what stops two writers reaching chrome.dll together.
func TestPatchLock_ConcurrentAcquireOnlyOneWins(t *testing.T) {
	dir := newUnpatchedCore(t, "mimic_149.2")

	const goroutines = 8
	var held, maxHeld atomic.Int32
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := acquirePatchLock(dir)
			if err != nil {
				t.Errorf("acquirePatchLock: %v", err)
				return
			}
			now := held.Add(1)
			if now > maxHeld.Load() {
				maxHeld.Store(now)
			}
			time.Sleep(5 * time.Millisecond)
			held.Add(-1)
			release()
		}()
	}
	wg.Wait()

	if got := maxHeld.Load(); got != 1 {
		t.Errorf("%d holders at once, want 1 — the lock is not exclusive", got)
	}
}
