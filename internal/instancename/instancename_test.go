package instancename

import "testing"

func TestDisplay(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Blood Gulch", "Blood Gulch"},
		{"mixed case + punctuation kept", "Stew's Box!", "Stew's Box!"},
		{"trims whitespace", "  hi there  ", "hi there"},
		{"drops non-ASCII (accents not in 0x20-0x7E)", "café", "caf"},
		{"drops control chars", "a\tb\nc", "abc"},
		{"caps at 15 chars", "way-too-long-container-name", "way-too-long-co"},
		{"exactly 15 kept", "123456789012345", "123456789012345"},
		{"16 truncated to 15", "1234567890123456", "123456789012345"},
		{"empty", "", ""},
		{"all non-printable -> empty", "\x00\x01\x1f", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Display(c.in)
			if got != c.want {
				t.Fatalf("Display(%q) = %q, want %q", c.in, got, c.want)
			}
			if len([]rune(got)) > MaxDisplay {
				t.Fatalf("Display(%q) length %d exceeds MaxDisplay %d", c.in, len([]rune(got)), MaxDisplay)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "BloodGulch", "bloodgulch"},
		{"space to dash", "Blood Gulch", "blood-gulch"},
		{"multiple spaces collapse", "Blood   Gulch", "blood-gulch"},
		{"punctuation dropped", "Stew's Box!", "stews-box"},
		{"keeps single . _ -", "v1.0_final-a", "v1.0_final-a"},
		{"collapses separator runs", "a__b--c..d", "a-b-c-d"},
		{"mixed separator run collapses", "a-_.b", "a-b"},
		{"trims leading/trailing separators", "--.hidden._-", "hidden"},
		{"already a slug", "beta-play-abc", "beta-play-abc"},
		{"digits ok", "map42", "map42"},
		{"empty", "", ""},
		{"only separators -> empty", "-_.-", ""},
		{"only punctuation -> empty", "!!!", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Slug(c.in); got != c.want {
				t.Fatalf("Slug(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSlugIsPodmanSafe: a slugged display name (optionally prefixed) is always a
// valid podman container name — first char alphanumeric, only [a-z0-9._-].
func TestSlugIsPodmanSafe(t *testing.T) {
	inputs := []string{"Blood Gulch", "Stew's Box!", "v1.0", "  spaced  ", "a__b", "-leading", "trailing-"}
	for _, in := range inputs {
		s := Slug(Display(in))
		if s == "" {
			continue // empty slug is the caller's fallback case, not invalid
		}
		if isSep(rune(s[0])) {
			t.Errorf("Slug(%q) = %q starts with a separator (invalid podman name)", in, s)
		}
		for _, r := range s {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
			if !ok {
				t.Errorf("Slug(%q) = %q contains invalid podman char %q", in, s, r)
			}
		}
		// With a prefix it's still valid (starts with 'b').
		full := "beta-" + s
		if isSep(rune(full[0])) {
			t.Errorf("prefixed %q invalid", full)
		}
	}
}
