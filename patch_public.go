package mimickit

import "github.com/tuwibu/mimickit/internal/standalone"

// PatchCore NOPs the mimic "initial check" gate in the given chrome.dll so a
// standalone (adapter-less) launch passes it. It is idempotent, backs the
// original up to <dll>.bak once, and fails safe when the signature is absent.
// Call it once post-install (after the core is downloaded/extracted), not on
// every launch — the DLL is ~100-200MB.
func PatchCore(chromeDLL string) error { return standalone.PatchCore(chromeDLL) }

// RestoreCore reverts chrome.dll from the backup written by PatchCore.
func RestoreCore(chromeDLL string) error { return standalone.RestoreCore(chromeDLL) }
