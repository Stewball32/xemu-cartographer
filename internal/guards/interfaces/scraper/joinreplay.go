package scraper

// JoinReplay exposes the bytes a newly-subscribed scraper-room client needs
// to be caught up after joining mid-match. The websocket join_room handler
// dispatches per room: host:<name> calls JoinReplayForInstance(name);
// host:all calls JoinReplayForHostAll. Without that replay, a late joiner
// only sees broadcasts going forward and the overlay UI never gets
// map / players / power-item-spawn data to render.
//
// Renamed from `Snapshot` / `LatestSnapshotMessages` in M5 stage 5a — the
// underlying bytes still encode the legacy `Type:"snapshot"` envelope
// (until M5 stage 5c replaces it with `current_state`), but the *method*
// describes its purpose ("messages a joiner needs") rather than the
// soon-to-retire wire type.
//
// JoinReplayMessages (the legacy "all instances" variant) is retained for
// the request_state handler until M5 stage 5d narrows it to a single room.
type JoinReplay interface {
	JoinReplayMessages() [][]byte
	JoinReplayForInstance(name string) [][]byte
	JoinReplayForHostAll() [][]byte
}
