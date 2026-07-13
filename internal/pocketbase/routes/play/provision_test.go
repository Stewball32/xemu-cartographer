package play

import "testing"

// TestInstanceName covers the container-naming decision for request-instance:
// non-admins always get a stable per-user name (can't override), admins may
// override with a sanitized name, and an empty/absent basis yields "".
func TestInstanceName(t *testing.T) {
	cases := []struct {
		name     string
		prefix   string
		userID   string
		override string
		isAdmin  bool
		want     string
	}{
		// prod: empty prefix (unchanged behavior).
		{"non-admin derives per-user", "", "abc123def456ghi", "", false, "play-abc123def456ghi"},
		{"non-admin cannot override", "", "abc123def456ghi", "custom-box", false, "play-abc123def456ghi"},
		{"admin may override", "", "abc123def456ghi", "smoke-1", true, "smoke-1"},
		{"admin empty override falls back to per-user", "", "abc123def456ghi", "", true, "play-abc123def456ghi"},
		{"admin override sanitized", "", "abc123def456ghi", "My Box!!", true, "mybox"},
		{"admin override that sanitizes to empty falls back", "", "abc123def456ghi", "!!!", true, "play-abc123def456ghi"},
		{"no userID and no usable override → empty", "", "", "", false, ""},
		{"userID sanitized in derivation", "", "AB-c.1", "", false, "play-ab-c.1"},
		// beta: non-empty prefix namespaces every derived name.
		{"beta prefixes per-user", "beta-", "abc123def456ghi", "", false, "beta-play-abc123def456ghi"},
		{"beta prefixes admin override", "beta-", "abc123def456ghi", "smoke-1", true, "beta-smoke-1"},
		{"beta empty basis stays empty (no bare prefix)", "beta-", "", "", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := instanceName(c.prefix, c.userID, c.override, c.isAdmin); got != c.want {
				t.Fatalf("instanceName(%q, %q, %q, admin=%v) = %q, want %q",
					c.prefix, c.userID, c.override, c.isAdmin, got, c.want)
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
