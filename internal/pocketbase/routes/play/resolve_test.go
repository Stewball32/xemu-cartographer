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
	name, ok := resolveContainer(false, "", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("gamertag stew should resolve pod1, got %q ok=%v", name, ok)
	}
	if _, ok := resolveContainer(false, "", []string{"nobody"}, view()); ok {
		t.Fatal("unmatched gamertag must not resolve a container")
	}
}

func TestResolveContainerAdminOverride(t *testing.T) {
	// Admin with an override targets any container regardless of gamertag.
	name, ok := resolveContainer(true, "pod2", nil, view())
	if !ok || name != "pod2" {
		t.Fatalf("admin override should resolve pod2, got %q ok=%v", name, ok)
	}
	// A non-admin override is ignored → falls back to gamertag match.
	name, ok = resolveContainer(false, "pod2", []string{"stew"}, view())
	if !ok || name != "pod1" {
		t.Fatalf("non-admin override must be ignored (match stew→pod1), got %q ok=%v", name, ok)
	}
	// Admin with no override still resolves by their own gamertag.
	name, ok = resolveContainer(true, "", []string{"zed"}, view())
	if !ok || name != "pod2" {
		t.Fatalf("admin without override should match own gamertag zed→pod2, got %q ok=%v", name, ok)
	}
}

func TestInCatalog(t *testing.T) {
	if !inCatalog(hostrunner.CEMaps, "Blood Gulch") {
		t.Fatal("Blood Gulch should be in the CE map catalog")
	}
	if inCatalog(hostrunner.CEMaps, "Zanzibar") {
		t.Fatal("Zanzibar (an H2 map) must not be in the CE map catalog")
	}
	if !inCatalog(hostrunner.CEGametypes, "Team Slayer") {
		t.Fatal("Team Slayer should be a CE gametype")
	}
}

// The Registry satisfies the PlayControl interface the routes call.
func TestRegistrySatisfiesPlayControl(t *testing.T) {
	var _ PlayControl = hostrunner.NewRegistry(nil)
}
