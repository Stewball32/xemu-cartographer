package haloce

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// stockMapList is the live mp_map_list ustr content read from a stock CE disc
// (RUNTIME-VERIFIED 2026-07-10 on ce-nav), including the two trailing
// non-selectable placeholder entries.
var stockMapList = []string{
	"Battle Creek", "Sidewinder", "Damnation", "Rat Race", "Prisoner",
	"Hang 'Em High", "Chill Out", "Derelict", "Boarding Action", "Blood Gulch",
	"Wizard", "Chiron TL34", "Longest", "Unknown Level", " ",
}

// stockGametypeNames is the live default_multiplayer_game_setting_names content.
var stockGametypeNames = []string{
	"Slayer", "Slayer Pro", "Elimination", "Phantoms", "Endurance", "Rockets",
	"Snipers", "Oddball", "Reverse Tag", "Accumulate", "Juggernaut", "Stalker",
	"King", "King Pro", "Crazy King", "Race", "Rally", "CTF", "Invasion",
	"Iron CTF", "CTF Pro", "Team Race", "Team Rally", "Team Ball", "Team King",
	"Team Slayer",
}

func steps(opts []scraper.LobbyOption, name string) (int, bool) {
	for _, o := range opts {
		if o.Name == name {
			return o.Steps, true
		}
	}
	return 0, false
}

func TestBuildMapOptions_SelectableAndCarouselOrder(t *testing.T) {
	// 13 descriptions on stock CE → placeholders 13/14 are dropped.
	got := buildMapOptions(stockMapList, 13)
	if len(got) != 13 {
		t.Fatalf("selectable map count = %d, want 13", len(got))
	}
	// Steps is the option's ABSOLUTE carousel index (position), preserving the
	// live mp_map_list order — NOT a press count from any assumed default.
	for i, name := range stockMapList[:13] {
		s, ok := steps(got, name)
		if !ok {
			t.Errorf("map %q missing from options", name)
			continue
		}
		if s != i {
			t.Errorf("map %q Steps(index) = %d, want %d", name, s, i)
		}
	}
	// The dropped placeholders must not appear.
	if _, ok := steps(got, "Unknown Level"); ok {
		t.Errorf("placeholder %q should have been dropped", "Unknown Level")
	}
}

func TestBuildMapOptions_NoDescriptionsTrimsBlankTail(t *testing.T) {
	// Descriptions unavailable (count 0): only the trailing blank is trimmed, so
	// "Unknown Level" survives (best-effort fallback).
	got := buildMapOptions(stockMapList, 0)
	if len(got) != 14 {
		t.Fatalf("count = %d, want 14 (drop only the trailing blank)", len(got))
	}
	if _, ok := steps(got, "Unknown Level"); !ok {
		t.Errorf("expected %q to survive when descriptions are unavailable", "Unknown Level")
	}
	if _, ok := steps(got, " "); ok {
		t.Errorf("trailing blank should be trimmed")
	}
}

func TestBuildGametypeOptions_CarouselOrder(t *testing.T) {
	got := buildGametypeOptions(stockGametypeNames)
	if len(got) != 26 {
		t.Fatalf("gametype count = %d, want 26", len(got))
	}
	// Steps is the option's absolute carousel index (position), in live order.
	for i, name := range stockGametypeNames {
		s, ok := steps(got, name)
		if !ok {
			t.Errorf("gametype %q missing", name)
			continue
		}
		if s != i {
			t.Errorf("gametype %q Steps(index) = %d, want %d", name, s, i)
		}
	}
}

func TestBuildOptions_AbsoluteIndex(t *testing.T) {
	names := []string{"m0", "m1", "m2", "m3", "m4"}
	got := buildOptions(names)
	for i, nm := range names {
		if s, _ := steps(got, nm); s != i {
			t.Errorf("%q Steps(index) = %d, want %d", nm, s, i)
		}
	}
	if buildOptions(nil) != nil {
		t.Errorf("buildOptions(nil) should be nil")
	}
}

func TestTrimTrailingBlank(t *testing.T) {
	in := []string{"a", "", "b", " ", "\t"}
	got := trimTrailingBlank(in)
	if len(got) != 3 || got[2] != "b" {
		t.Errorf("trimTrailingBlank = %v, want [a  b]", got)
	}
	// Interior blanks are preserved (Steps alignment).
	if got[1] != "" {
		t.Errorf("interior blank should be preserved, got %q", got[1])
	}
}

func TestLeU32(t *testing.T) {
	b := []byte{0x78, 0x56, 0x34, 0x12, 0xFF}
	if v := leU32(b, 0); v != 0x12345678 {
		t.Errorf("leU32 = 0x%X, want 0x12345678", v)
	}
	// Out of range → 0.
	if v := leU32(b, 3); v != 0 {
		t.Errorf("leU32 out-of-range = 0x%X, want 0", v)
	}
}
