package routes

import (
	"reflect"
	"testing"
)

func TestParseGamertagList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		max  int
		want []string
	}{
		{"empty", "", 10, []string{}},
		{"trims + drops empties", " Stewball , , gravemind ", 10, []string{"Stewball", "gravemind"}},
		{"case-insensitive dedupe, first wins", "Stew,stew,STEW", 10, []string{"Stew"}},
		{"caps at max", "a,b,c,d", 2, []string{"a", "b"}},
		{"only commas", ",,,", 10, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseGamertagList(c.raw, c.max)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("parseGamertagList(%q, %d) = %v, want %v", c.raw, c.max, got, c.want)
			}
		})
	}
}

func TestUserAvatarPath(t *testing.T) {
	if got := userAvatarPath("abc123", "me_x7.png"); got != "/api/files/users/abc123/me_x7.png?thumb=100x100" {
		t.Fatalf("userAvatarPath = %q", got)
	}
	for _, empty := range []string{"", "  "} {
		if got := userAvatarPath("abc123", empty); got != "" {
			t.Fatalf("userAvatarPath(%q) = %q, want ''", empty, got)
		}
	}
}

func TestCeColorFromSettings(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"color present", `{"color": 2, "thumbstick": 1}`, 2},
		{"missing color", `{"thumbstick": 1}`, 0},
		{"empty", "", 0},
		{"null", "null", 0},
		{"garbage", "not json", 0},
		{"float coerced", `{"color": 11.0}`, 11},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ceColorFromSettings(c.in); got != c.want {
				t.Fatalf("ceColorFromSettings(%q) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestAppearanceFromJSON(t *testing.T) {
	t.Run("decodes byte map, drops out-of-range + non-numeric", func(t *testing.T) {
		in := `{"armor_primary": 2, "emblem_foreground": 12, "bad": 300, "worse": "x"}`
		got := appearanceFromJSON(in)
		want := map[string]int{"armor_primary": 2, "emblem_foreground": 12}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("appearanceFromJSON = %v, want %v", got, want)
		}
	})
	for _, in := range []string{"", "null", "{}", "  "} {
		if got := appearanceFromJSON(in); got != nil {
			t.Fatalf("appearanceFromJSON(%q) = %v, want nil", in, got)
		}
	}
}

func TestSanitizedKey(t *testing.T) {
	cases := []struct{ sanitized, tag, want string }{
		{"stewball32", "Stewball32", "stewball32"}, // sanitized column wins
		{"", "  OG50 II  ", "og50 ii"},             // falls back to a normalised tag
		{"  ", "MixedCase", "mixedcase"},           // blank sanitized is not a match key
		{"", "", ""},                               // nothing to match on
		{"  Padded  ", "other", "padded"},          // sanitized is normalised too
	}
	for _, c := range cases {
		if got := sanitizedKey(c.sanitized, c.tag); got != c.want {
			t.Errorf("sanitizedKey(%q,%q) = %q, want %q", c.sanitized, c.tag, got, c.want)
		}
	}
}

func TestDisplayNameFor(t *testing.T) {
	cases := []struct {
		name                             string
		defTag, defStatus, matched, want string
	}{
		{"approved default wins", "Stewart", "approved", "Stewball32", "Stewart"},
		{"allowed default also wins", "Stewart", "allowed", "Stewball32", "Stewart"},
		// The whole point of gating: an unreviewed or blocked default must not
		// reach the stream just because another tag of theirs was approved.
		{"pending default falls back", "NotYetReviewed", "pending", "Stewball32", "Stewball32"},
		{"blocked default falls back", "Naughty", "blocked", "Stewball32", "Stewball32"},
		{"no default at all", "", "", "Stewball32", "Stewball32"},
		{"blank approved default falls back", "   ", "approved", "Stewball32", "Stewball32"},
		{"matched tag is trimmed", "", "", "  Stewball32  ", "Stewball32"},
	}
	for _, c := range cases {
		if got := displayNameFor(c.defTag, c.defStatus, c.matched); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestProfileVisibleStatuses(t *testing.T) {
	// A broadcast shows only affirmatively-reviewed names. `pending` is excluded
	// deliberately — stricter than the status != "blocked" rule used for matching
	// and authorization elsewhere.
	for _, s := range []string{"approved", "allowed"} {
		if !profileVisibleStatuses[s] {
			t.Errorf("status %q must be broadcast-visible", s)
		}
	}
	for _, s := range []string{"pending", "blocked", "", "APPROVED"} {
		if profileVisibleStatuses[s] {
			t.Errorf("status %q must NOT be broadcast-visible", s)
		}
	}
}
