package rooms

import (
	"strings"
	"testing"
)

// TestRoomForInstance covers the reserved-name chokepoint. Every code path
// that derives a room name from an instance name routes through
// RoomForInstance / RoomForInstanceClass, so input-validation rules
// belong here and only here.
func TestRoomForInstance(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string // substring match; empty means no error expected
	}{
		{name: "simple", input: "smoke1", want: "host:smoke1"},
		{name: "single-char", input: "x", want: "host:x"},
		{name: "alphanumeric", input: "abc-123_xyz", want: "host:abc-123_xyz"},

		{name: "empty", input: "", wantErr: "name required"},
		{name: "reserved-all", input: "all", wantErr: "reserved"},
		{name: "reserved-summary", input: "summary", wantErr: "reserved"},
		{name: "contains-colon", input: "foo:bar", wantErr: `must not contain ':'`},
		{name: "contains-space", input: "foo bar", wantErr: "whitespace"},
		{name: "contains-tab", input: "foo\tbar", wantErr: "whitespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RoomForInstance(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("RoomForInstance(%q): want error containing %q, got nil (result %q)", tt.input, tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("RoomForInstance(%q): error %q does not contain %q", tt.input, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RoomForInstance(%q): unexpected error %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("RoomForInstance(%q): want %q, got %q", tt.input, tt.want, got)
			}
		})
	}
}

// TestHostRoomResolves verifies that the host RoomType is registered and
// that per-instance, host:all, host:summary, and per-class room names all
// resolve to it (the registry's prefix:suffix Resolve strips at the first
// ":").
func TestHostRoomResolves(t *testing.T) {
	cases := []string{
		"host:smoke1",
		HostAllRoom,
		SummaryRoom,
		"host:smoke1:tick",
		"host:smoke1:debug",
	}
	for _, room := range cases {
		rt, ok := Resolve(room)
		if !ok {
			t.Fatalf("Resolve(%q): not found — host RoomType must register itself via init()", room)
		}
		if rt.Name != HostRoomPrefix {
			t.Fatalf("Resolve(%q).Name = %q, want %q", room, rt.Name, HostRoomPrefix)
		}
	}
}

// TestRoomForInstanceClass covers per-class room naming (v2). Instance
// validation reuses RoomForInstance's rules; class must be in the known
// scraper class set.
func TestRoomForInstanceClass(t *testing.T) {
	tests := []struct {
		name     string
		instance string
		class    string
		want     string
		wantErr  string
	}{
		{name: "tick", instance: "smoke1", class: "tick", want: "host:smoke1:tick"},
		{name: "debug", instance: "smoke1", class: "debug", want: "host:smoke1:debug"},
		{name: "game", instance: "pod-2", class: "game", want: "host:pod-2:game"},
		{name: "previous_game", instance: "pod-2", class: "previous_game", want: "host:pod-2:previous_game"},

		{name: "empty-instance", instance: "", class: "tick", wantErr: "name required"},
		{name: "reserved-all", instance: "all", class: "tick", wantErr: "reserved"},
		{name: "reserved-summary", instance: "summary", class: "tick", wantErr: "reserved"},
		{name: "unknown-class", instance: "smoke1", class: "bogus", wantErr: "not a known scraper class"},
		{name: "summary-not-per-instance", instance: "smoke1", class: "summary", wantErr: "not a known scraper class"},
		{name: "hello-not-class", instance: "smoke1", class: "hello", wantErr: "not a known scraper class"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RoomForInstanceClass(tt.instance, tt.class)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("RoomForInstanceClass(%q, %q): want error containing %q, got nil (result %q)", tt.instance, tt.class, tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("RoomForInstanceClass(%q, %q): error %q does not contain %q", tt.instance, tt.class, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RoomForInstanceClass(%q, %q): unexpected error %v", tt.instance, tt.class, err)
			}
			if got != tt.want {
				t.Fatalf("RoomForInstanceClass(%q, %q): want %q, got %q", tt.instance, tt.class, tt.want, got)
			}
		})
	}
}
