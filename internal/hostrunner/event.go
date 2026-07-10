package hostrunner

// RunnerEvent is one entry on the observable input stream (requirement 5): the
// high-level player INTENT plus the actual KEY the runner emitted, along with
// the read-only native start conditions. Broadcast to the admin WS room so the
// kiosk can show what "players" are sending.
type RunnerEvent struct {
	Instance  string `json:"instance"`
	Tick      uint32 `json:"tick"`
	Authority string `json:"authority"`
	Screen    string `json:"screen"`

	// Kind mirrors Action.Kind ("tap"/"chord"/"wait"/"blocked"/"done"), plus
	// "suspended" when the runner stood down for an admin.
	Kind   string   `json:"kind"`
	Intent string   `json:"intent,omitempty"`
	Keys   []string `json:"keys,omitempty"`
	Reason string   `json:"reason,omitempty"`

	// Native start conditions — READ, never controlled — always surfaced so the
	// UI can show why start is (not) happening.
	MachineCount    int  `json:"machine_count"`
	TeamCount       int  `json:"team_count"`
	PlayerCount     int  `json:"player_count"`
	CountdownActive bool `json:"countdown_active"`
	ReadyToStart    bool `json:"ready_to_start"`
}

// EventSink receives the observable stream. The prod adapter serialises to JSON
// and broadcasts to the admin room via svc.WS.SendToRoomRaw; tests use a fake.
type EventSink interface {
	Emit(RunnerEvent)
}

// NopSink discards events (used when no observer is wired).
type NopSink struct{}

func (NopSink) Emit(RunnerEvent) {}

// buildEvent stamps an Action + observation into a RunnerEvent.
func buildEvent(instance string, obs Observation, auth Authority, kind string, act Action) RunnerEvent {
	return RunnerEvent{
		Instance:        instance,
		Tick:            obs.Tick,
		Authority:       auth.String(),
		Screen:          Classify(obs).String(),
		Kind:            kind,
		Intent:          act.Intent,
		Keys:            act.Keys,
		Reason:          act.Reason,
		MachineCount:    obs.MachineCount,
		TeamCount:       obs.TeamCount,
		PlayerCount:     obs.PlayerCount,
		CountdownActive: obs.CountdownActive,
		ReadyToStart:    obs.ReadyToStart(),
	}
}
