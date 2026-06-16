package handlers

import "testing"

func TestHostRoomInstance(t *testing.T) {
	cases := map[string]string{
		"host:pod-a":      "pod-a",
		"host:pod-a:game": "pod-a",
		"host:pod-a:xbox": "pod-a",
		"host:summary":    "summary", // a literal segment; summary handled separately
		"host":            "",
		"admin:dashboard": "",
		"public:lobby":    "",
		"":                "",
	}
	for room, want := range cases {
		if got := hostRoomInstance(room); got != want {
			t.Errorf("hostRoomInstance(%q) = %q, want %q", room, got, want)
		}
	}
}

// authorizeHostRoom's admin / roster paths need a live PB app; here we cover
// the branches that don't: non-host rooms are passed through, and the summary
// feed is denied to a caller with no admin context.
func TestAuthorizeHostRoom_NonHostPassThrough(t *testing.T) {
	for _, room := range []string{"admin:dashboard", "public:lobby", "", "overlay:x"} {
		e := &Event{Room: room}
		if err := authorizeHostRoom(e); err != nil {
			t.Errorf("authorizeHostRoom(room=%q) = %v, want nil (non-host pass-through)", room, err)
		}
	}
}

func TestAuthorizeHostRoom_SummaryDeniedWithoutAdmin(t *testing.T) {
	// nil Services → no admin context → summary stays admin-only.
	for _, room := range []string{"host:summary", "host:all"} {
		e := &Event{Room: room}
		if err := authorizeHostRoom(e); err == nil {
			t.Errorf("authorizeHostRoom(room=%q) = nil, want denial (admin-only summary)", room)
		}
	}
}

func TestAuthorizeHostRoom_InstanceDeniedWithoutContext(t *testing.T) {
	// nil Services → can't resolve roster → per-instance room denied (fail closed).
	e := &Event{Room: "host:pod-a"}
	if err := authorizeHostRoom(e); err == nil {
		t.Error("authorizeHostRoom(host:pod-a, nil services) = nil, want denial")
	}
}
