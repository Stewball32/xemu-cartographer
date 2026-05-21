package manager

// envelopeTypeSummary is the wire type for the cross-instance summary
// class — one entry per running runner, suitable for a dashboard. No
// `instance` on the envelope (it's multi-instance by design); the
// aggregator broadcasts to a dedicated `host:summary` room.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`summary`).
const envelopeTypeSummary = "summary"

// SummaryPayload wraps the existing hostSummary list (defined in
// aggregator.go) in an object so the payload has somewhere to grow
// (server-wide metrics, instance counts, etc.) without breaking
// consumers.
type SummaryPayload struct {
	Hosts []hostSummary `json:"hosts"`
}
