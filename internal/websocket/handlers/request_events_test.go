package handlers

import (
	"testing"

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
// per-sender rooms, and a SendRaw recorder.
func requestEvent(stub *stubRequestScraper, senderRooms []string, sent *[]string) *Event {
	e := &Event{
		Services: &guards.Services{Scraper: stub},
		SendRaw:  func(data []byte) { *sent = append(*sent, string(data)) },
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
