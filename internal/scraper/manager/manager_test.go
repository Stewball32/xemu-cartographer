package manager

import (
	"testing"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
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

	if got := m.List(); len(got) != 0 {
		t.Fatalf("empty manager List(): want 0 entries, got %d", len(got))
	}

	if err := m.Stop("nonexistent"); err != nil {
		t.Fatalf("Stop(nonexistent): want nil, got %v", err)
	}
}

// TestStartRequiresNameAndSock guards the input validation in Start so a typo
// in the route handler can't crash the server with a nil-deref deeper in.
func TestStartRequiresNameAndSock(t *testing.T) {
	m := New(nil)

	if err := m.Start("", "/tmp/sock"); err == nil {
		t.Fatal("Start with empty name: want error, got nil")
	}
	if err := m.Start("name", ""); err == nil {
		t.Fatal("Start with empty sock: want error, got nil")
	}
}

// fakeReader is a no-op GameReader used to inject a runner without standing up
// a real xemu instance. Only Title() and XboxName() are exercised by InstanceState.
type fakeReader struct{ title, xbox string }

func (f *fakeReader) LowGVAs() []uint32                                 { return nil }
func (f *fakeReader) ReadGameState() (scraper.GameState, uint32, error) { return "", 0, nil }
func (f *fakeReader) LastStateInputs() scraper.StateInputs              { return nil }
func (f *fakeReader) BuildScoreProbe() scraper.ScoreProbe                { return nil }
func (f *fakeReader) ReadSnapshot() (scraper.SnapshotPayload, error) {
	return scraper.SnapshotPayload{}, nil
}
func (f *fakeReader) ReadLobby() (scraper.SnapshotPayload, error) {
	return scraper.SnapshotPayload{}, nil
}
func (f *fakeReader) ReadTick([]scraper.PowerItemSpawn, *scraper.TickState) (scraper.TickResult, error) {
	return scraper.TickResult{}, nil
}
func (f *fakeReader) DetectEvents(uint32, string, scraper.SnapshotPayload, scraper.TickResult, *scraper.TickState) []scraper.Envelope {
	return nil
}
func (f *fakeReader) OnStateChange(prev, next scraper.GameState) error { return nil }
func (f *fakeReader) NewTickState() *scraper.TickState                 { return scraper.NewTickState() }
func (f *fakeReader) XboxName() string                                 { return f.xbox }
func (f *fakeReader) Title() string                                    { return f.title }

func TestInstanceState(t *testing.T) {
	m := New(nil)

	// Missing name: zeroed InstanceState (with name echoed back) and ok=false.
	got, ok := m.InstanceState("nope")
	if ok {
		t.Fatalf("InstanceState(missing): want ok=false, got true")
	}
	if got.Name != "nope" || got.GameTitle != "" || got.XboxName != "" || got.Running {
		t.Fatalf("InstanceState(missing): unexpected payload %+v", got)
	}

	// With a runner injected, fields propagate from the GameReader.
	reader := &fakeReader{title: "Halo: Combat Evolved", xbox: "MyXbox"}
	r := newRunner("alpha", "/tmp/sock", 0x4D530004, nil, reader)
	defer r.cancel()
	m.runners["alpha"] = r

	got, ok = m.InstanceState("alpha")
	if !ok {
		t.Fatalf("InstanceState(present): want ok=true, got false")
	}
	if got.Name != "alpha" || got.TitleID != 0x4D530004 || got.GameTitle != "Halo: Combat Evolved" || got.XboxName != "MyXbox" || !got.Running {
		t.Fatalf("InstanceState(present): unexpected payload %+v", got)
	}
}
