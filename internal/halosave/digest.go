package halosave

import "errors"

// DigestNote summarises everything known about the unresolved 20-byte save
// digest. Kept verbatim from the de-risk so it can be surfaced in API responses
// and logs without re-deriving it.
const DigestNote = "20-byte content-dependent digest. NOT a plain SHA-1/MD5/CRC of any " +
	"contiguous range, NOT HMAC-SHA1 under obvious keys (tested exhaustively on " +
	"real samples). Most likely the Xbox content signature (HMAC-SHA1 under a " +
	"per-title key derived from the Halo XBE) or a Halo-internal salted hash. " +
	"Preserved from the template until cracked; enforcement-on-load is unverified."

// ErrDigestUnresolved is returned by RecomputeDigest until the digest algorithm
// is cracked. Callers should fall back to template-patch (preserve) and confirm
// acceptance via the xemu runtime probe before relying on edited-field files.
var ErrDigestUnresolved = errors.New("halosave: save digest algorithm not yet cracked — " + DigestNote)

// DigestMode records how a generated file's digest was produced.
type DigestMode string

const (
	// DigestPreserved means the 20 digest bytes were copied verbatim from the
	// template (the default template-patch behaviour). A file built this way is
	// byte-identical to its template when no fields changed; with edited fields
	// the digest will not match the new content — acceptable only if the game
	// does not re-verify the digest on load (the open runtime question).
	DigestPreserved DigestMode = "preserved-from-template"
	// DigestRecomputed means RecomputeDigest produced a fresh, correct digest.
	// Not reachable until the algorithm/key is implemented below.
	DigestRecomputed DigestMode = "recomputed"
)

// DigestStatus describes the digest handling of a generated SaveSet. It is
// JSON-serialisable so the API and the editor UIs can show, plainly, that an
// edited-settings file carries a stale (template) digest and whether that is a
// concern yet.
type DigestStatus struct {
	Mode     DigestMode `json:"mode"`
	Resolved bool       `json:"resolved"`
	// Edited is true when at least one understood field was changed relative to
	// the template, so the preserved digest no longer matches the content.
	Edited bool   `json:"edited"`
	Note   string `json:"note"`
}

// preservedDigest returns the DigestStatus for a template-patched file.
func preservedDigest(edited bool) DigestStatus {
	return DigestStatus{
		Mode:     DigestPreserved,
		Resolved: false,
		Edited:   edited,
		Note:     DigestNote,
	}
}

// RecomputeDigest is the pluggable hook for the unresolved 20-byte digest.
//
// buf is the full file with the (stale) digest still in place; off/length
// locate the digest field (CE: 0x68/20; H2: trailing 20). title is "ce" or
// "h2" so a future implementation can select the per-title key. It must return
// a length-byte digest to splice in at off.
//
// It is intentionally a no-op-failing stub: the algorithm is a keyed/salted
// SHA-1-family digest whose secret we do not have offline. When the runtime
// probe confirms the digest IS enforced and the signing key is recovered,
// implement it here and return (digest, nil); Build* will then splice it in
// when called with Recompute=true. Until then it returns ErrDigestUnresolved
// and Build* defaults to template-patch (preserve).
func RecomputeDigest(buf []byte, off, length int, title string) ([]byte, error) {
	return nil, ErrDigestUnresolved
}

// DigestResolved reports whether RecomputeDigest has been implemented. The API
// surfaces this so clients know whether edited-field files carry a valid digest.
func DigestResolved() bool {
	_, err := RecomputeDigest(make([]byte, 64), 0, 20, "ce")
	return err == nil
}
