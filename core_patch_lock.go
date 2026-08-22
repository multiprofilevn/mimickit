package mimickit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Patching a core rewrites chrome.dll in place — a few hundred MB read, two
// bytes changed, the whole file written back. Two launches racing on the same
// cold core would interleave those writes and leave a corrupt core behind, and
// the automation app opens profiles in bulk, so that race is ordinary operation
// rather than an edge case.
//
// Two layers guard it, because there are two ways to collide: goroutines inside
// one process, and separate processes (the desktop app and the automation agent
// share a core dir on the same machine).
const (
	patchLockName = ".mimic-patch.lock"

	// patchLockStaleAfter is how long a lock file may sit untouched before it is
	// treated as abandoned by a dead process. A real patch takes seconds even on
	// the largest core, so minutes of silence means nobody is coming back.
	patchLockStaleAfter = 5 * time.Minute

	// patchLockWait bounds how long we wait for another process to finish before
	// giving up, and patchLockPoll is how often we re-check.
	patchLockWait = 2 * time.Minute
	patchLockPoll = 50 * time.Millisecond
)

var (
	patchMuMapMu sync.Mutex
	patchMuMap   = map[string]*sync.Mutex{}
)

// corePatchMutex returns the in-process lock for one core directory, creating it
// on first use. Locking per directory rather than globally lets two different
// cores patch at the same time.
func corePatchMutex(coreDir string) *sync.Mutex {
	key := filepath.Clean(coreDir)

	patchMuMapMu.Lock()
	defer patchMuMapMu.Unlock()
	mu, ok := patchMuMap[key]
	if !ok {
		mu = &sync.Mutex{}
		patchMuMap[key] = mu
	}
	return mu
}

// acquirePatchLock takes the cross-process lock for coreDir and returns the
// function that releases it. Callers must invoke that function on every exit
// path — a lock left behind blocks every other process for patchLockStaleAfter.
func acquirePatchLock(coreDir string) (func(), error) {
	path := filepath.Join(coreDir, patchLockName)
	deadline := time.Now().Add(patchLockWait)

	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("acquire patch lock %s: %w", path, err)
		}

		// Someone holds it. Reclaim it if they are plainly gone; a lock whose
		// file vanished between Stat and now just means they finished, and the
		// next loop takes it.
		if fi, statErr := os.Stat(path); statErr == nil {
			if time.Since(fi.ModTime()) > patchLockStaleAfter {
				_ = os.Remove(path)
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("patch lock %s held for over %s", path, patchLockWait)
		}
		time.Sleep(patchLockPoll)
	}
}
