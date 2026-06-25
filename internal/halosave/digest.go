package halosave

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
)

// DigestNote summarises the (now resolved) 20-byte save digest. Kept so it can
// be surfaced in API responses and logs.
//
// CRACKED 2026-06-25: the digest is the Original-Xbox *roamable* save signature
//
//	signature(20B) = HMAC-SHA1(AuthKey, data)      data = buf[:sigOffset]
//	AuthKey(16B)   = HMAC-SHA1(XboxGlobalKey, cert.sig_key)[:16]
//
// The signing key is derived from a global Xbox constant and the *per-title*
// certificate signature key from the game's XBE — there is NO console-specific
// input (no XboxHDKey / EEPROM). The signature is therefore portable across
// consoles ("roamable"), so the hub can sign centrally. Verified byte-exact
// against every real sample (CE 23/23, H2 3/3) and self-consistent on edits.
const DigestNote = "20-byte Original-Xbox roamable save signature: " +
	"HMAC-SHA1(AuthKey, buf[:sigOffset]), AuthKey = HMAC-SHA1(XboxGlobalKey, title_cert_sig_key)[:16]. " +
	"Per-title (roamable) — no per-console key needed; signed centrally by the hub. " +
	"Verified byte-exact on all real samples (CE 23/23, H2 3/3)."

// ErrDigestUnresolved is returned by RecomputeDigest only for a title whose
// per-title signing key we do not have embedded (CE and H2 are both resolved).
var ErrDigestUnresolved = errors.New("halosave: no embedded signing key for title")

// xboxGlobalKey is the global "Xbox key" baked into the Xbox OS (public; used by
// the standard roamable-save signature derivation). Same constant used by every
// title; the per-title key below is what specialises the signature.
var xboxGlobalKey = mustHex("5C0733AE0401F7E8BA7993FDCD2F1FE0")

// titleSigKeys are the 16-byte certificate signature keys (plaintext @cert+0xC0
// in the retail XBEs). Cross-checked against the real Halo CE/H2 default.xbe
// certs (byte-exact). Used to derive the per-title AuthKey.
var titleSigKeys = map[string][]byte{
	TitleCE: mustHex("1F71DE93D52AADB19446D7494F731158"), // Halo CE, title 0x4D530004
	TitleH2: mustHex("2116D927510F01D19B7EC75CAFE669AC"), // Halo 2,  title 0x4D530064
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("halosave: bad embedded key hex: " + err.Error())
	}
	return b
}

// deriveAuthKey computes the per-title 16-byte signing key:
// AuthKey = HMAC-SHA1(xboxGlobalKey, sigKey)[:16].
func deriveAuthKey(sigKey []byte) []byte {
	m := hmac.New(sha1.New, xboxGlobalKey)
	m.Write(sigKey)
	return m.Sum(nil)[:16]
}

// DigestMode records how a generated file's digest was produced.
type DigestMode string

const (
	// DigestPreserved means the 20 digest bytes were copied verbatim from the
	// template. Only used now as a fallback for titles without an embedded key.
	DigestPreserved DigestMode = "preserved-from-template"
	// DigestRecomputed means RecomputeDigest produced a fresh, correct digest.
	// This is the default for CE and H2 generated files.
	DigestRecomputed DigestMode = "recomputed"
)

// DigestStatus describes the digest handling of a generated SaveSet. It is
// JSON-serialisable so the API and editor UIs can show that an edited file
// carries a correctly recomputed (valid) signature.
type DigestStatus struct {
	Mode     DigestMode `json:"mode"`
	Resolved bool       `json:"resolved"`
	// Edited is true when at least one understood field was changed relative to
	// the template (informational; the digest is valid either way).
	Edited bool   `json:"edited"`
	Note   string `json:"note"`
}

// recomputedDigest returns the DigestStatus for a correctly re-signed file.
func recomputedDigest(edited bool) DigestStatus {
	return DigestStatus{
		Mode:     DigestRecomputed,
		Resolved: true,
		Edited:   edited,
		Note:     DigestNote,
	}
}

// preservedDigest returns the DigestStatus for a template-patched file whose
// digest was not recomputed (fallback for unknown titles only).
func preservedDigest(edited bool) DigestStatus {
	return DigestStatus{
		Mode:     DigestPreserved,
		Resolved: false,
		Edited:   edited,
		Note:     DigestNote,
	}
}

// RecomputeDigest produces the correct 20-byte Xbox roamable save signature for
// buf and returns the length-byte digest to splice in at off.
//
// buf is the full file with the (stale) digest still in place; off/length
// locate the digest field (CE: 0x68/20; H2 profile: 0x1E0/20; H2 gametype:
// trailing 20). The signed message is everything before the signature field,
// buf[:off] — correct for CE (sig embedded at 0x68 over the 104 preceding
// bytes; the trailing padding is not signed) and for H2 (trailing sig over all
// preceding bytes). title selects the per-title key ("ce" or "h2").
func RecomputeDigest(buf []byte, off, length int, title string) ([]byte, error) {
	if off < 0 || length < 0 || off+length > len(buf) {
		return nil, fmt.Errorf("halosave: digest field %#x+%d out of range for %d-byte buffer", off, length, len(buf))
	}
	if length > sha1.Size {
		return nil, fmt.Errorf("halosave: digest length %d exceeds SHA-1 size %d", length, sha1.Size)
	}
	sigKey, ok := titleSigKeys[title]
	if !ok {
		return nil, fmt.Errorf("%w %q (have %v)", ErrDigestUnresolved, title, titleKeys())
	}
	auth := deriveAuthKey(sigKey)
	m := hmac.New(sha1.New, auth)
	m.Write(buf[:off])
	return m.Sum(nil)[:length], nil
}

// DigestResolved reports whether RecomputeDigest can sign for CE and H2. The API
// surfaces this so clients know generated edited files carry a valid signature.
func DigestResolved() bool {
	for _, t := range []string{TitleCE, TitleH2} {
		if _, err := RecomputeDigest(make([]byte, 64), 0, 20, t); err != nil {
			return false
		}
	}
	return true
}

func titleKeys() []string {
	out := make([]string, 0, len(titleSigKeys))
	for k := range titleSigKeys {
		out = append(out, k)
	}
	return out
}
