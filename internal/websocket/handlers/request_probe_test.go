package handlers

import (
	"encoding/json"
	"testing"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/authztest"
)

func TestHandleRequestProbe_SenderRooms(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		senderRooms []string // nil models a missing Rooms capability
		wantAsked   []string
	}{
		{
			// Explicit instance short-circuits before the room walk — a nil
			// Rooms capability proves membership is never consulted.
			name:        "explicit instance skips room walk",
			payload:     `{"instance":"pod-x"}`,
			senderRooms: nil,
			wantAsked:   []string{"pod-x"},
		},
		{
			name:        "fallback walks sender rooms with dedup",
			senderRooms: []string{"host:pod-a", "host:pod-a:game", "host:pod-b", "host:all", "host:summary"},
			wantAsked:   []string{"pod-a", "pod-b"},
		},
		{
			name:        "no instance and nil Rooms capability fails closed",
			senderRooms: nil,
			wantAsked:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubRequestScraper{}
			var sent []string
			e := requestEvent(stub, tc.senderRooms, &sent)
			if tc.payload != "" {
				e.Payload = json.RawMessage(tc.payload)
			}
			handleRequestProbe(e)
			if !equalStrings(stub.probeAsked, tc.wantAsked) {
				t.Errorf("ProbeReply asked for %q, want %q", stub.probeAsked, tc.wantAsked)
			}
			if len(sent) != len(tc.wantAsked) {
				t.Errorf("sent %d replies (%q), want %d", len(sent), sent, len(tc.wantAsked))
			}
		})
	}
}

// TestHandleRequestProbe_DeniedWithoutScope: the probe exposes raw memory
// reads, so authz.Can(scraper.probe, Instance) is scope-only — a roster
// line, box ownership or a binding never grants it, and neither the
// explicit-instance path nor the room walk asks the scraper for a denied
// instance. Nothing is sent back either way.
func TestHandleRequestProbe_DeniedWithoutScope(t *testing.T) {
	rooms := []string{"host:pod-a", "host:pod-b:tick"}
	deps := &authztest.FakeDeps{
		Rostered: map[string]bool{authztest.Key("u1", "pod-a"): true},
		Owned:    map[string]string{"u1": "pod-b"},
	}
	tests := []struct {
		name      string
		p         authz.Principal
		deps      authz.Deps
		payload   string
		wantAsked []string
	}{
		{"machine without scope, explicit", machine("k1"), deps, `{"instance":"pod-a"}`, nil},
		{"machine without scope, walk", machine("k1"), deps, "", nil},
		{"machine events-only scope", machine("k2", "scraper.events:*"), deps, `{"instance":"pod-a"}`, nil},
		{"machine scoped to one instance, explicit other", machine("k3", "scraper.probe:pod-b"), deps, `{"instance":"pod-a"}`, nil},
		{"machine scoped to one instance, walk", machine("k3", "scraper.probe:pod-b"), deps, "", []string{"pod-b"}},
		{"rostered box owner without scope, explicit", pbUser("u1"), deps, `{"instance":"pod-a"}`, nil},
		{"rostered box owner without scope, walk", pbUser("u1"), deps, "", nil},
		{"admin user", pbUser("a1", adminScopes...), deps, "", []string{"pod-a", "pod-b"}},
		{"spectator bound with scope", boundKey(authz.KindSpectator, "s1", "pod-a", "scraper.*"), deps, `{"instance":"pod-a"}`, nil},
		{"device bound with scope", boundKey(authz.KindDevice, "d1", "pod-a", "scraper.*"), deps, "", nil},
		{"anonymous bound", authz.Anonymous("pod-a", []string{"scraper.*"}), deps, `{"instance":"pod-a"}`, nil},
		{"nobody", authz.Nobody(), deps, "", nil},
		{"superuser with nil deps, explicit", authz.Superuser("su"), nil, `{"instance":"pod-a"}`, nil},
		{"superuser with nil deps, walk", authz.Superuser("su"), nil, "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubRequestScraper{}
			var sent []string
			e := requestEvent(stub, rooms, &sent)
			e.Principal, e.Authz = tc.p, tc.deps
			if tc.payload != "" {
				e.Payload = json.RawMessage(tc.payload)
			}
			handleRequestProbe(e)
			if !equalStrings(stub.probeAsked, tc.wantAsked) {
				t.Errorf("ProbeReply asked for %q, want %q", stub.probeAsked, tc.wantAsked)
			}
			if len(sent) != len(tc.wantAsked) {
				t.Errorf("sent %d replies (%q), want %d", len(sent), sent, len(tc.wantAsked))
			}
		})
	}
}
