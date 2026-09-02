package manager

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"time"
)

// newGameUID mints the per-captured-game idempotency key: 32 lowercase hex
// chars encoding 16 bytes — a 6-byte big-endian unix-milliseconds timestamp
// followed by 10 bytes of crypto/rand. Time-sortable like a ULID/UUIDv7
// without pulling in a dependency; consumers (internal/games dedupe, the
// stats pipeline) treat it as opaque.
//
// Called exactly once per captured game (captureLiveAsPrevious), so every
// re-delivery of the same previous_game — join replay, request_state, the
// persistence hook — carries the same uid.
func newGameUID() string {
	var b [16]byte
	ms := uint64(time.Now().UnixMilli())
	binary.BigEndian.PutUint16(b[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(b[2:6], uint32(ms))
	// crypto/rand.Read never returns an error (Go 1.24+ guarantee).
	_, _ = rand.Read(b[6:])
	return hex.EncodeToString(b[:])
}
