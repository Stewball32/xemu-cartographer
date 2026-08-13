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

func TestResolveContainerAdminOverride(t *testing.T) {
	// Admin with an override targets any container regardless of gamertag.
	name, ok := resolveContainer(true, "pod2", "", nil, view())
	if !ok || name != "pod2" {
		t.Fatalf("admin override should resolve pod2, got %q ok=%v", name, ok)
	}
	// A non-admin override is ignored → falls back to gamertag match.
	name, ok = resolveContainer(false, "pod2", "", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("non-admin override must be ignored (match stew→pod1), got %q ok=%v", name, ok)
	}
	// Admin with no override still resolves by their own gamertag.
	name, ok = resolveContainer(true, "", "", []string{"zed"}, view())
	if !ok || name != "pod2" {
		t.Fatalf("admin without override should match own gamertag zed→pod2, got %q ok=%v", name, ok)
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
