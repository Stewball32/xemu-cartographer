package handlers

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// stubRequestScraper satisfies scraperiface.Service via the embedded (nil)
// interface and records which instances the request handlers asked about.
// Shared with request_probe_test.go.
type stubRequestScraper struct {
	scraperiface.Service
	eventsAsked []string
	probeAsked  []string
}

func (s *stubRequestScraper) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	s.eventsAsked = append(s.eventsAsked, instance)
	return []byte("events " + instance), true
}

func (s *stubRequestScraper) ProbeReply(instance string) ([]byte, bool) {
	s.probeAsked = append(s.probeAsked, instance)
	return []byte("probe " + instance), true
}

// requestEvent builds an Event wired to the stub scraper, the given
// per-sender rooms, a SendRaw recorder, an empty FakeDeps and a machine
// principal holding scraper.* — the credential the request handlers were
// written for (a scraper-side key), so membership is the only variable
// in the SenderRooms tests. The DeniedWithoutScope siblings swap p out.
func requestEvent(stub *stubRequestScraper, senderRooms []string, sent *[]string) *Event {
	e := &Event{
		Services:  &guards.Services{Scraper: stub},
		Authz:     &authztest.FakeDeps{},
		Principal: machine("scraper-key", "scraper.*"),
		SendRaw:   func(data []byte) { *sent = append(*sent, string(data)) },
	}
	if senderRooms != nil {
		e.Rooms = func() []string { return senderRooms }
	}
	return e
}

func TestHandleRequestEvents_SenderRooms(t *testing.T) {
	tests := []struct {
		name        string
		senderRooms []string // nil models a missing Rooms capability
		wantAsked   []string
	}{
		{
			name:        "dedup per instance, aggregates skipped",
			senderRooms: []string{"host:pod-a:event", "host:pod-a:tick", "host:pod-b", "host:all", "host:summary", "admin:dashboard"},
			wantAsked:   []string{"pod-a", "pod-b"},
		},
		{
			name:        "no rooms, no replies",
			senderRooms: []string{},
			wantAsked:   nil,
		},
		{
			name:        "nil Rooms capability fails closed",
			senderRooms: nil,
			wantAsked:   nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubRequestScraper{}
			var sent []string
			handleRequestEvents(requestEvent(stub, tc.senderRooms, &sent))
			if !equalStrings(stub.eventsAsked, tc.wantAsked) {
				t.Errorf("EventsReply asked for %q, want %q", stub.eventsAsked, tc.wantAsked)
			}
			if len(sent) != len(tc.wantAsked) {
				t.Errorf("sent %d replies (%q), want %d", len(sent), sent, len(tc.wantAsked))
			}
		})
	}
}

// TestHandleRequestEvents_DeniedWithoutScope: room membership selects the
// instances but authz.Can(scraper.events, Instance) decides each one. A
// principal that is in the room yet cannot present the scope (or, for a
// user, a roster line / box ownership; for a bound key, its binding) is
// skipped silently — no reply, no error, and the scraper is never asked.
func TestHandleRequestEvents_DeniedWithoutScope(t *testing.T) {
	rooms := []string{"host:pod-a:event", "host:pod-b:tick"}
	deps := &authztest.FakeDeps{Rostered: map[string]bool{authztest.Key("u1", "pod-a"): true}}
	tests := []struct {
		name      string
		p         authz.Principal
		deps      authz.Deps
		wantAsked []string
	}{
		{"machine without scope", machine("k1"), deps, nil},
		{"machine with unrelated scope", machine("k2", "room.join:*"), deps, nil},
		{"machine scoped to one instance", machine("k3", "scraper.events:pod-b"), deps, []string{"pod-b"}},
		{"user rostered in one instance", pbUser("u1"), deps, []string{"pod-a"}},
		{"user rostered nowhere", pbUser("u2"), deps, nil},
		{"user with scraper scope", pbUser("u3", "scraper.*"), deps, []string{"pod-a", "pod-b"}},
		{"spectator bound with scope", boundKey(authz.KindSpectator, "s1", "pod-b", "scraper.events:*"), deps, []string{"pod-b"}},
		{"spectator bound without scope", boundKey(authz.KindSpectator, "s2", "pod-b"), deps, nil},
		{"device bound elsewhere", boundKey(authz.KindDevice, "d1", "pod-z", "scraper.*"), deps, nil},
		{"anonymous bound", authz.Anonymous("pod-a", []string{"scraper.*"}), deps, nil},
		{"nobody", authz.Nobody(), deps, nil},
		{"superuser with nil deps", authz.Superuser("su"), nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubRequestScraper{}
			var sent []string
			e := requestEvent(stub, rooms, &sent)
			e.Principal, e.Authz = tc.p, tc.deps
			handleRequestEvents(e)
			if !equalStrings(stub.eventsAsked, tc.wantAsked) {
				t.Errorf("EventsReply asked for %q, want %q", stub.eventsAsked, tc.wantAsked)
			}
			if len(sent) != len(tc.wantAsked) {
				t.Errorf("sent %d replies (%q), want %d", len(sent), sent, len(tc.wantAsked))
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
