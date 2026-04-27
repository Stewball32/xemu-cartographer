package scraper

// Snapshot exposes the most recent cached snapshot broadcast for each running
// scraper, wrapped as a websocket.Message JSON []byte ready to send. The
// websocket join_room handler uses this to replay snapshots to clients that
// subscribe to the overlay room mid-match (see ROADMAP.md "Snapshot replay
// for late joiners").
type Snapshot interface {
	LatestSnapshotMessages() [][]byte
}
