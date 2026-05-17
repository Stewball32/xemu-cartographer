package manager

import (
	"testing"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// TestManagerSatisfiesInterface verifies that *Manager structurally implements
// scraperiface.Service. If this fails to compile, Services.Scraper assignments
// in main.go will also fail — catching it here gives a clearer error than the
// downstream failure.
func TestManagerSatisfiesInterface(t *testing.T) {
	var _ scraperiface.Service = (*Manager)(nil)
}

// TestEmptyManagerListAndStop covers the no-runners branches of List and Stop:
// fresh manager returns an empty list, and stopping an unknown name is a no-op
// (returns nil rather than ErrNotFound).
func TestEmptyManagerListAndStop(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if got := m.List(); len(got) != 0 {
		t.Fatalf("empty manager List(): want 0 entries, got %d", len(got))
	}

	if err := m.Stop("nonexistent"); err != nil {
		t.Fatalf("Stop(nonexistent): want nil, got %v", err)
	}
}

// TestStartRequiresNameAndSock guards the input validation in Start so a typo
// in the route handler can't crash the server with a nil-deref deeper in.
// M5 stage 5b: empty / reserved / colonised names are rejected by the
// rooms.RoomForInstance chokepoint; this test exercises the full chokepoint
// behaviour rather than just the legacy empty-string check.
func TestStartRequiresNameAndSock(t *testing.T) {
	m := New(nil)
	defer m.Close()

	if err := m.Start("", "/tmp/sock"); err == nil {
		t.Fatal("Start with empty name: want error, got nil")
	}
	if err := m.Start("name", ""); err == nil {
		t.Fatal("Start with empty sock: want error, got nil")
	}
	if err := m.Start("all", "/tmp/sock"); err == nil {
		t.Fatal(`Start with reserved name "all": want error, got nil`)
	}
	if err := m.Start("foo:bar", "/tmp/sock"); err == nil {
		t.Fatal(`Start with name containing ":": want error, got nil`)
	}
}

func TestInstanceState(t *testing.T) {
	m := New(nil)
	defer m.Close()

	// Missing name: zeroed InstanceState (with name echoed back) and ok=false.
	got, ok := m.InstanceState("nope")
	if ok {
		t.Fatalf("InstanceState(missing): want ok=false, got true")
	}
	if got.Name != "nope" || got.Title != "" || got.XboxName != "" || got.Running {
		t.Fatalf("InstanceState(missing): unexpected payload %+v", got)
	}

	// With a runner injected, fields read out of the runner's cache. M5
	// stage 5a: identity values (TitleID, Title, XboxName) live on
	// instanceCache rather than on the runner directly, populated by the
	// loop when it binds a reader. Simulate that here by writing to the
	// cache directly so InstanceState has data to surface.
	r := newRunner("alpha", "/tmp/sock", "host:alpha", nil, nil)
	defer r.cancel()
	r.cache.TitleID = 0x4D530004
	r.cache.Title = "Halo: Combat Evolved"
	r.cache.XboxName = "MyXbox"
	m.runners["alpha"] = r

	got, ok = m.InstanceState("alpha")
	if !ok {
		t.Fatalf("InstanceState(present): want ok=true, got false")
	}
	if got.Name != "alpha" || got.TitleID != 0x4D530004 || got.Title != "Halo: Combat Evolved" || got.XboxName != "MyXbox" || !got.Running {
		t.Fatalf("InstanceState(present): unexpected payload %+v", got)
	}
}
