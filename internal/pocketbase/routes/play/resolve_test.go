package play

import (
	"testing"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
)

func view() []scraperiface.ContainerMembership {
	return []scraperiface.ContainerMembership{
		{Container: "pod1", Identities: []string{"stew", "console-a"}},
		{Container: "pod2", Identities: []string{"zed"}},
	}
}

func TestResolveContainerGamertagMatch(t *testing.T) {
	name, ok := resolveContainer(false, "", "", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("gamertag stew should resolve pod1, got %q ok=%v", name, ok)
	}
	if _, ok := resolveContainer(false, "", "", []string{"nobody"}, view()); ok {
		t.Fatal("unmatched gamertag must not resolve a container")
	}
}

// TestResolveContainerOverrideAllowed: a caller the rule table lets control
// the override container (canOverride) targets it regardless of gamertag; with
// no override the same caller still resolves by their own gamertag.
func TestResolveContainerOverrideAllowed(t *testing.T) {
	name, ok := resolveContainer(true, "pod2", "", nil, view())
	if !ok || name != "pod2" {
		t.Fatalf("allowed override should resolve pod2, got %q ok=%v", name, ok)
	}
	// canOverride with no override is moot — own gamertag still applies.
	name, ok = resolveContainer(true, "", "", []string{"zed"}, view())
	if !ok || name != "pod2" {
		t.Fatalf("no override should match own gamertag zed→pod2, got %q ok=%v", name, ok)
	}
	// An override the caller may control wins over a roster match elsewhere.
	name, ok = resolveContainer(true, "pod2", "", []string{"stew"}, view())
	if !ok || name != "pod2" {
		t.Fatalf("allowed override must beat the roster match, got %q ok=%v", name, ok)
	}
}

// TestResolveContainerOverrideDenied: a denied override (no box.control on it)
// is ignored — never honoured — and resolution falls through to ownership /
// the roster match, or idle when neither applies.
func TestResolveContainerOverrideDenied(t *testing.T) {
	name, ok := resolveContainer(false, "pod2", "", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("denied override must be ignored (match stew→pod1), got %q ok=%v", name, ok)
	}
	// Ownership still resolves under a denied override.
	v := append(view(), scraperiface.ContainerMembership{Container: "play-uid123"})
	name, ok = resolveContainer(false, "pod2", "play-uid123", nil, v)
	if !ok || name != "play-uid123" {
		t.Fatalf("denied override should fall through to the owned box, got %q ok=%v", name, ok)
	}
	// Nothing to fall through to: idle, never the override.
	if name, ok := resolveContainer(false, "pod2", "", nil, view()); ok || name != "" {
		t.Fatalf("denied override with no ownership/roster must be idle, got %q ok=%v", name, ok)
	}
	// An override the caller cannot control must not leak through even when it
	// names a live container.
	if name, ok := resolveContainer(false, "pod1", "", []string{"zed"}, view()); !ok || name != "pod2" {
		t.Fatalf("denied override must not pick pod1; expected zed→pod2, got %q ok=%v", name, ok)
	}
}

// TestResolveContainerOwnership: the caller's own per-user box resolves by NAME
// the moment it exists — before anyone is in its roster (fresh boot) and even if
// nobody ever is (neutral host). Roster matching still covers joining someone
// else's box; ownership wins over a coincidental roster match elsewhere.
func TestResolveContainerOwnership(t *testing.T) {
	v := append(view(), scraperiface.ContainerMembership{
		Container: "beta-play-uid123", Identities: nil, // fresh box: EMPTY roster
	})
	// Owner resolves their booting box with no gamertag match anywhere.
	name, ok := resolveContainer(false, "", "beta-play-uid123", nil, v)
	if !ok || name != "beta-play-uid123" {
		t.Fatalf("owned box should resolve by name, got %q ok=%v", name, ok)
	}
	// Ownership beats a roster match in a different container.
	name, ok = resolveContainer(false, "", "beta-play-uid123", []string{"stew"}, v)
	if !ok || name != "beta-play-uid123" {
		t.Fatalf("ownership should take priority, got %q ok=%v", name, ok)
	}
	// Owned box not (yet) provisioned → falls through to roster match.
	name, ok = resolveContainer(false, "", "beta-play-uid123", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("absent owned box should fall back to roster, got %q ok=%v", name, ok)
	}
	// Nothing owned, nothing matched → idle.
	if _, ok := resolveContainer(false, "", "beta-play-uid123", nil, view()); ok {
		t.Fatal("no owned box + no match must stay idle")
	}
}

// The live map list drives selection-nav Steps by carousel index — there is no
// hardcoded/stock table anywhere in the path.
func TestMapListIndexOf(t *testing.T) {
	list := scraperiface.MapList{
		Available: true,
		Maps: []scraperiface.MapOption{
			{Name: "battlecreek", Steps: 0},
			{Name: "custom_modded_map", Steps: 1}, // a modded disc's map — must be enumerable
			{Name: "bloodgulch", Steps: 2},
		},
	}
	if steps, ok := list.IndexOf(list.Maps, "bloodgulch"); !ok || steps != 2 {
		t.Fatalf("bloodgulch should navigate 2 steps, got %d ok=%v", steps, ok)
	}
	if steps, ok := list.IndexOf(list.Maps, "custom_modded_map"); !ok || steps != 1 {
		t.Fatalf("a modded map must be found in the live list, got %d ok=%v", steps, ok)
	}
	if _, ok := list.IndexOf(list.Maps, "not_on_this_disc"); ok {
		t.Fatal("a map not on this instance must not resolve")
	}
}

// The Registry satisfies the PlayControl interface the routes call.
func TestRegistrySatisfiesPlayControl(t *testing.T) {
	var _ PlayControl = hostrunner.NewRegistry(nil)
}
