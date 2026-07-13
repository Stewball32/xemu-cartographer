package play

import "testing"

// TestInstanceName covers the DECOUPLED naming decision for request-instance:
// non-admins always get a stable per-user container (no pretty display), admins
// may pass a pretty display name (≤15, printable ASCII) whose slug becomes the
// container name, and an empty/absent basis yields an empty container.
func TestInstanceName(t *testing.T) {
	cases := []struct {
		name          string
		prefix        string
		userID        string
		override      string
		isAdmin       bool
		wantDisplay   string
		wantContainer string
	}{
		// prod: empty prefix.
		{"non-admin: per-user box, no display", "", "abc123def456ghi", "", false, "", "play-abc123def456ghi"},
		{"non-admin cannot override", "", "abc123def456ghi", "custom-box", false, "", "play-abc123def456ghi"},
		{"admin override → display + slug container", "", "abc123def456ghi", "smoke-1", true, "smoke-1", "smoke-1"},
		{"admin empty override falls back to per-user", "", "abc123def456ghi", "", true, "", "play-abc123def456ghi"},
		{"admin pretty name: display kept, container slugged", "", "abc123def456ghi", "My Box!!", true, "My Box!!", "my-box"},
		{"admin all-punctuation: display kept, container uid-fallback", "", "abc123def456ghi", "!!!", true, "!!!", "play-abc123def456ghi"},
		{"no userID and no usable override → empty container", "", "", "", false, "", ""},
		{"userID slug in derivation", "", "AB-c.1", "", false, "", "play-ab-c.1"},
		// beta: non-empty prefix namespaces the CONTAINER only, never the display.
		{"beta per-user box", "beta-", "abc123def456ghi", "", false, "", "beta-play-abc123def456ghi"},
		{"beta admin override", "beta-", "abc123def456ghi", "smoke-1", true, "smoke-1", "beta-smoke-1"},
		{"beta empty basis → empty container", "beta-", "", "", false, "", ""},
		// 15-char cap: a long pretty name truncates to 15 for display, slug follows.
		{"admin long name truncated to 15 + slugged", "beta-", "uid", "Way Too Long Name Here", true, "Way Too Long Na", "beta-way-too-long-na"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotDisplay, gotContainer := instanceName(c.prefix, c.userID, c.override, c.isAdmin)
			if gotDisplay != c.wantDisplay || gotContainer != c.wantContainer {
				t.Fatalf("instanceName(%q,%q,%q,admin=%v) = (%q,%q), want (%q,%q)",
					c.prefix, c.userID, c.override, c.isAdmin,
					gotDisplay, gotContainer, c.wantDisplay, c.wantContainer)
			}
			if len([]rune(gotDisplay)) > 15 {
				t.Errorf("display %q exceeds 15 chars", gotDisplay)
			}
		})
	}
}

// TestSanitizeName covers the podman-safe name filter.
func TestSanitizeName(t *testing.T) {
	cases := map[string]string{
		"abc123":        "abc123",
		"ABC":           "abc",
		"  spaced  ":    "spaced",
		"a b/c\\d":      "abcd",
		"under_score.-": "under_score.-",
		"!!!":           "",
		"héllo":         "hllo",
	}
	for in, want := range cases {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
