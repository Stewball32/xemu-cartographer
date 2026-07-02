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
