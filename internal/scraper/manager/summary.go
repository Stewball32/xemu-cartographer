package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The summary class is the cross-instance feed — one entry per running
// runner, suitable for a dashboard. No `instance` on the envelope (it's
// multi-instance by design); the aggregator broadcasts to the dedicated
// `host:summary` room.
//
// Shapes are owned by the wire contract package (xc-scraper/wire/summary.go).
// hostSummary keeps its historical unexported name here as an alias of the
// exported wire.HostSummary.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`summary`).

// hostSummary is one entry in the summary aggregate cache. Lean on purpose
// — summary subscribers are list views (admin debug index, future host
// picker UI), not per-instance overlays. Anything heavier belongs on the
// host:<name> per-instance stream.
type hostSummary = wire.HostSummary

// SummaryPayload wraps the hostSummary list in an object so the payload has
// somewhere to grow (server-wide metrics, instance counts, etc.) without
// breaking consumers.
type SummaryPayload = wire.SummaryPayload
