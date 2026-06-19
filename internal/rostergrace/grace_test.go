package rostergrace

import (
	"testing"
	"time"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

func view(container string, ids ...string) []scraperiface.ContainerMembership {
	return []scraperiface.ContainerMembership{{Container: container, Identities: ids}}
}

var base = time.Unix(1_700_000_000, 0)

func TestAllow_PresentGrants(t *testing.T) {
	tr := New(5 * time.Minute)
	if !tr.Allow(view("pod-a", "stewie"), "pod-a", []string{"stewie"}, base) {
		t.Error("a tag present in the roster should be allowed")
	}
}

func TestAllow_NeverSeenDenied(t *testing.T) {
	tr := New(5 * time.Minute)
	if tr.Allow(view("pod-a"), "pod-a", []string{"stewie"}, base) {
		t.Error("a never-seen, absent tag should be denied")
	}
}

func TestAllow_EmptyTagsDenied(t *testing.T) {
	tr := New(5 * time.Minute)
	if tr.Allow(view("pod-a", "stewie"), "pod-a", nil, base) {
		t.Error("empty tags should be denied")
	}
	if tr.Allow(view("pod-a", "stewie"), "pod-a", []string{""}, base) {
		t.Error("blank tag should be denied")
	}
}

func TestAllow_GraceWindowKeepsThenRevokes(t *testing.T) {
	tr := New(5 * time.Minute)
	present := view("pod-a", "stewie")
	absent := view("pod-a") // roster no longer lists stewie (e.g. editing gametype)

	// Seen live at base.
	if !tr.Allow(present, "pod-a", []string{"stewie"}, base) {
		t.Fatal("should be allowed when present")
	}
	// Absent but inside the window → still allowed (the usability win).
	if !tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(4*time.Minute)) {
		t.Error("within the grace window, an absent tag should still be allowed")
	}
	// Absent past the window (5m TTL from the last live sighting) → revoked.
	if tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(6*time.Minute)) {
		t.Error("past the grace window, access should be revoked (fail-closed)")
	}
}

func TestAllow_BoundaryInclusive(t *testing.T) {
	tr := New(5 * time.Minute)
	tr.Allow(view("pod-a", "stewie"), "pod-a", []string{"stewie"}, base)
	// Exactly at the TTL boundary is still allowed (<= ttl).
	if !tr.Allow(view("pod-a"), "pod-a", []string{"stewie"}, base.Add(5*time.Minute)) {
		t.Error("exactly at the TTL boundary should still be allowed")
	}
}

func TestAllow_LiveSightingRefreshesWindow(t *testing.T) {
	tr := New(5 * time.Minute)
	present := view("pod-a", "stewie")
	absent := view("pod-a")

	tr.Allow(present, "pod-a", []string{"stewie"}, base)                    // sighting @ base
	tr.Allow(present, "pod-a", []string{"stewie"}, base.Add(4*time.Minute)) // sighting @ +4 (refresh)

	// +8m is 8m after the first sighting but only 4m after the refresh → allowed.
	if !tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(8*time.Minute)) {
		t.Error("a live sighting should refresh the window")
	}
	// +10m is 6m after the last sighting (@ +4) → revoked.
	if tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(10*time.Minute)) {
		t.Error("should revoke 6m after the last live sighting")
	}
}

func TestAllow_GraceDoesNotRefresh(t *testing.T) {
	// A removed gamertag that keeps polling (absent every time) must NOT extend
	// its own access — only live presence refreshes.
	tr := New(5 * time.Minute)
	absent := view("pod-a")

	tr.Allow(view("pod-a", "stewie"), "pod-a", []string{"stewie"}, base) // last live sighting @ base
	// Polls while absent within the window — allowed, but no refresh.
	tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(3*time.Minute))
	tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(4*time.Minute))
	// 6m after the last LIVE sighting → revoked despite the polling.
	if tr.Allow(absent, "pod-a", []string{"stewie"}, base.Add(6*time.Minute)) {
		t.Error("grace-granted access must not refresh the window")
	}
}

func TestAllow_PerContainerNoLeak(t *testing.T) {
	tr := New(5 * time.Minute)
	// Seen in pod-a only.
	tr.Allow(view("pod-a", "stewie"), "pod-a", []string{"stewie"}, base)
	// Must not grant access to a different container.
	if tr.Allow(view("pod-b"), "pod-b", []string{"stewie"}, base.Add(time.Minute)) {
		t.Error("grace is per-container; a pod-a sighting must not grant pod-b")
	}
}

func TestAllow_MultiTagAnyGrants(t *testing.T) {
	tr := New(5 * time.Minute)
	// One of the caller's two tags is present.
	if !tr.Allow(view("pod-a", "alt"), "pod-a", []string{"main", "alt"}, base) {
		t.Error("any present tag should grant access")
	}
}
