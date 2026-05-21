// Package sinks defines the destination interface for capture-policy
// payloads and the file-backed implementation. See
// atlas/new_json/04-ground-up-rebuild.md §5 ("sinks") for the design
// rationale: per-(instance, class) policy rows in capture_policies
// carry a `sink` spec string ("file:replays/{instance}/tick.ndjson",
// later "pb:game_events"), which the runner parses into a Sink and tees
// envelope bytes to on every demand-gated broadcast.
//
// Sinks are owned by one runner. The scraper.manager package coordinates
// open / write / close around capture-policy reloads and runner
// lifecycle — this package is just the transport.
package sinks

import "io"

// Sink consumes one v2 envelope per Write call. The bytes are the
// marshaled scraper.Envelope (the inner payload, not the websocket
// Message wrapper) so a sink file can be replayed line-by-line and
// each line unmarshals straight into scraper.Envelope.
//
// Write may be called from the runner's loop goroutine on every poll;
// implementations should buffer / flush however they like as long as
// the call is non-blocking enough to keep up with engine cadence.
//
// Close is idempotent — sinkManager calls it on policy change AND on
// runner shutdown, and either may happen first.
type Sink interface {
	Write(envelopeBytes []byte) error
	io.Closer
}
