package hostrunner

import (
	"testing"
	"time"
)

func TestRegistryStatusAndControl(t *testing.T) {
	var downstream fakeSink
	reg := NewRegistry(&downstream)

	r := New(Config{Instance: "pod1"}, &fakeInput{}, reg) // sink = registry
	reg.Register("pod1", r)

	// Drive one tick so there's a last event.
	r.Tick(systemLink(), time.Unix(1000, 0))

	st := reg.Status("pod1")
	if !st.Present || st.Authority != "runner" {
		t.Fatalf("status: present=%v auth=%q", st.Present, st.Authority)
	}
	if st.Screen != "system_link" {
		t.Errorf("status screen = %q, want system_link", st.Screen)
	}
	if len(downstream.events) == 0 {
		t.Error("downstream sink should have received the event")
	}

	// Control: take over → authority reflects it.
	if !reg.SetAuthority("pod1", AuthAdmin) {
		t.Fatal("SetAuthority should succeed for known instance")
	}
	if reg.Status("pod1").Authority != "admin" {
		t.Fatal("authority should be admin after SetAuthority")
	}

	// Unknown instance.
	if reg.SetAuthority("nope", AuthAdmin) {
		t.Fatal("SetAuthority should fail for unknown instance")
	}
	if reg.Status("nope").Present {
		t.Fatal("unknown instance should not be present")
	}

	reg.Remove("pod1")
	if _, ok := reg.Get("pod1"); ok {
		t.Fatal("runner should be removed")
	}
}

func TestParseAuthority(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want Authority
		ok   bool
	}{
		{"runner", AuthRunner, true},
		{"admin", AuthAdmin, true},
		{"disabled", AuthDisabled, true},
		{"bogus", AuthRunner, false},
	} {
		got, ok := ParseAuthority(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("ParseAuthority(%q) = %v,%v want %v,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestScraperReadoutObservation(t *testing.T) {
	ro := ScraperReadout{
		Fresh: true, Tick: 42, GameState: "menu", MenuActive: true, GameConnection: 1,
		Map: "bloodgulch", Gametype: "slayer", MachineCount: 2, TeamCount: 2, CountdownActive: true,
	}
	obs := ro.Observation()
	if !obs.Fresh || obs.Phase != PhaseMenu || obs.Connection != ConnSystemLink {
		t.Fatalf("bad projection: %+v", obs)
	}
	if Classify(obs) != ScreenSystemLink {
		t.Errorf("classify = %v, want system_link", Classify(obs))
	}
	// Unknown gamestate + out-of-range connection clamp to safe defaults.
	bad := ScraperReadout{Fresh: true, GameState: "weird", GameConnection: 99}.Observation()
	if bad.Phase != PhaseMenu || bad.Connection != ConnMenu {
		t.Errorf("bad values should clamp to menu, got phase=%v conn=%v", bad.Phase, bad.Connection)
	}
}
