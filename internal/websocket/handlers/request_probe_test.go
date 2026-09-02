package handlers

import (
	"encoding/json"
	"testing"
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
