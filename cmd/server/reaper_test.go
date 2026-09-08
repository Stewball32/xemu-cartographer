package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/reaper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// storeGame caches one game frame for name in m (seq 1, now).
func storeGame(t *testing.T, m *xcclient.Mirror, name string, phase wire.Phase, machines int) {
	t.Helper()
	p := wire.GamePayload{Phase: phase}
	for i := 0; i < machines; i++ {
		p.Machines = append(p.Machines, wire.GameMachine{Index: i, Name: "xbox"})
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	env := &wire.Envelope{V: 1, Type: "game", Instance: name, Seq: 1, Ts: time.Now(), Data: data}
	raw, _ := json.Marshal(env)
	m.Store(name, "game", raw, env)
}

func activeOf(snaps []reaper.Snapshot) map[string]bool {
	out := make(map[string]bool, len(snaps))
	for _, s := range snaps {
		out[s.Instance] = s.Active
	}
	return out
}

// Wire-mode reaper source (DESIGN-STEP8 §9): phase / machines from the
// mirror's summary + game frames, host-runner machine count via the daemon.
func TestMirrorReaperSourceSnapshot(t *testing.T) {
	m := xcclient.NewMirror()
	now := time.Now()
	m.SetInstance("play-idle", now, false)
	storeGame(t, m, "play-idle", wire.PhaseReady, 1)
	storeGame(t, m, "play-live", wire.PhaseLive, 1)
	storeGame(t, m, "play-lobby", wire.PhaseReady, 2)
	storeGame(t, m, "play-hosted", wire.PhaseReady, 1)
	m.SetInstance("play-bare", now, true) // summary-only instance, no game frame yet

	hostCalls := map[string]int{}
	src := mirrorReaperSource{
		mirror: m,
		host: func(name string) hostrunner.Status {
			hostCalls[name]++
			if name == "play-hosted" {
				return hostrunner.Status{Instance: name, Present: true, MachineCount: 2}
			}
			return hostrunner.Status{Instance: name}
		},
	}

	got := activeOf(src.Snapshot())
	want := map[string]bool{
		"play-idle":   false,
		"play-live":   true,
		"play-lobby":  true,
		"play-hosted": true,
		"play-bare":   false,
	}
	if len(got) != len(want) {
		t.Fatalf("snapshot names = %v, want %v", got, want)
	}
	for name, active := range want {
		if got[name] != active {
			t.Errorf("%s: active = %v, want %v", name, got[name], active)
		}
	}
	// Mirror signals decide first; the daemon's /host route is consulted only
	// for boxes that are otherwise idle (one call per poll each).
	for _, name := range []string{"play-live", "play-lobby"} {
		if hostCalls[name] != 0 {
			t.Errorf("%s: /host polled %d times although the mirror already says active", name, hostCalls[name])
		}
	}
	for _, name := range []string{"play-idle", "play-hosted", "play-bare"} {
		if hostCalls[name] != 1 {
			t.Errorf("%s: /host polled %d times, want 1", name, hostCalls[name])
		}
	}
}

func TestMirrorReaperSourceSummaryPhaseWins(t *testing.T) {
	m := xcclient.NewMirror()
	storeGame(t, m, "play-a", wire.PhaseReady, 1) // stale per-instance frame
	storeGame(t, m, "play-b", wire.PhaseLive, 1)
	sum := &wire.SummaryPayload{Hosts: []wire.HostSummary{
		{Instance: "play-a", Phase: wire.PhaseLive},
		{Instance: "play-b", Phase: wire.PhaseIdle},
	}}
	raw, _ := json.Marshal(sum)
	m.StoreSummary(raw, sum)

	src := mirrorReaperSource{mirror: m} // hostrunner off: host nil, never consulted
	got := activeOf(src.Snapshot())
	if !got["play-a"] {
		t.Error("play-a: summary says live, must be active")
	}
	if got["play-b"] {
		t.Error("play-b: summary says idle and one machine, must be idle")
	}
}

func TestMirrorReaperSourceEmptyMirror(t *testing.T) {
	src := mirrorReaperSource{mirror: xcclient.NewMirror()}
	if snaps := src.Snapshot(); len(snaps) != 0 {
		t.Fatalf("empty mirror ⇒ no snapshots, got %v", snaps)
	}
}
