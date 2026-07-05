// Package standalone holds the adapter-less launch mechanism for mimic chrome:
// the chrome.dll gate patch, the AES fingerprint server, the chrome argument
// builder, and the plugin-handshake env. It is a LEAF package — it must not
// import the root mimickit package, so the root launcher can import it without a
// cycle. Fingerprint mapping (which depends on root types) lives in the root
// package and feeds this package a flat Config.
package standalone

import (
	"bytes"
	"fmt"
	"os"
)

// gateSig is the epilogue of mimic chrome's startup "initial check":
//
//	mov bl,1 ; test rax,rax ; je <fail> ; mov rax,[rsp+..]
//
// The check only passes when chrome was launched by the genuine adapter. NOPing
// the conditional jump makes the failure branch unreachable so a standalone
// launch always passes. The signature is version-specific (chrome 149.2); a
// different build will not match and PatchCore fails safe without touching bytes.
var gateSig = []byte{0xB3, 0x01, 0x48, 0x85, 0xC0, 0x74, 0x39, 0x48, 0x8B, 0x84, 0x24}

// gatePatched is gateSig with the `je` (74 39) already replaced by two NOPs
// (90 90) — used to recognize an already-patched DLL for idempotency.
var gatePatched = []byte{0xB3, 0x01, 0x48, 0x85, 0xC0, 0x90, 0x90, 0x48, 0x8B, 0x84, 0x24}

// jeOffset is the distance from the signature start to the `je` opcode bytes.
const jeOffset = 5

// PatchCore NOPs the mimic initial-check gate in chrome.dll so a standalone
// launch passes. It is idempotent (a second call on an already-patched DLL is a
// no-op), backs the original up to <dll>.bak exactly once, and fails safe: if the
// signature is absent the file is left untouched and an error is returned.
func PatchCore(dll string) error {
	data, err := os.ReadFile(dll)
	if err != nil {
		return err
	}
	idx := bytes.Index(data, gateSig)
	if idx < 0 {
		if bytes.Contains(data, gatePatched) {
			return nil // already patched
		}
		return fmt.Errorf("gate signature not found in %s (chrome.dll version differs)", dll)
	}

	bak := dll + ".bak"
	if _, statErr := os.Stat(bak); os.IsNotExist(statErr) {
		if err := os.WriteFile(bak, data, 0o644); err != nil {
			return fmt.Errorf("write backup %s: %w", bak, err)
		}
	}

	je := idx + jeOffset
	data[je], data[je+1] = 0x90, 0x90
	if err := os.WriteFile(dll, data, 0o644); err != nil {
		return fmt.Errorf("write patched %s: %w", dll, err)
	}
	return nil
}

// RestoreCore reverts chrome.dll from the <dll>.bak backup written by PatchCore.
// It errors clearly when no backup exists and leaves the .bak in place.
func RestoreCore(dll string) error {
	bak := dll + ".bak"
	b, err := os.ReadFile(bak)
	if err != nil {
		return fmt.Errorf("no backup %s: %w", bak, err)
	}
	if err := os.WriteFile(dll, b, 0o644); err != nil {
		return fmt.Errorf("restore %s: %w", dll, err)
	}
	return nil
}
