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
