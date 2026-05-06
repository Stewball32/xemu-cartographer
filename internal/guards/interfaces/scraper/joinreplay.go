package scraper

// JoinReplay exposes the bytes a newly-subscribed scraper-room client needs
// to be caught up after joining mid-match. The websocket join_room handler
// dispatches per room: host:<name> calls JoinReplayForInstance(name);
// host:all calls JoinReplayForHostAll. Without that replay, a late joiner
// only sees broadcasts going forward and the overlay UI never gets
// map / players / power-item-spawn data to render.
//
// Renamed from `Snapshot` / `LatestSnapshotMessages` in M5 stage 5a; the
// underlying bytes encode the M5 stage 5c `current_state` envelope —
// a full instanceCache snapshot for host:<name> joiners, and the
// hostsCache list for host:all joiners.
//
// JoinReplayMessages (the legacy "all instances" variant) is retained for
// the request_state handler until M5 stage 5d narrows it to a single room.
type JoinReplay interface {
	JoinReplayMessages() [][]byte
	JoinReplayForInstance(name string) [][]byte
	JoinReplayForHostAll() [][]byte
}
