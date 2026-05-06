package rooms

import (
	"strings"
	"testing"
)

// TestRoomForInstance covers the M5 stage 5b reserved-name chokepoint. Every
// code path that derives a room name from an instance name routes through
// RoomForInstance, so input-validation rules belong here and only here.
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
// that both per-instance and host:all room names resolve to it (the
// registry's prefix:suffix Resolve strips at the first ":").
func TestHostRoomResolves(t *testing.T) {
	rt, ok := Resolve("host:smoke1")
	if !ok {
		t.Fatal(`Resolve("host:smoke1"): not found — host RoomType must register itself via init()`)
	}
	if rt.Name != HostRoomPrefix {
		t.Fatalf(`Resolve("host:smoke1").Name = %q, want %q`, rt.Name, HostRoomPrefix)
	}

	rt2, ok := Resolve(HostAllRoom)
	if !ok {
		t.Fatalf("Resolve(%q): not found", HostAllRoom)
	}
	if rt2.Name != HostRoomPrefix {
		t.Fatalf("Resolve(%q).Name = %q, want %q (host:all and host:<name> share one RoomType)", HostAllRoom, rt2.Name, HostRoomPrefix)
	}
}
