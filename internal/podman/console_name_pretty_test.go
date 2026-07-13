package podman

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/consolename"
)

// TestConsoleNameFor: the console-nickname write uses the pretty DISPLAY name
// when present, falling back to the container name otherwise.
func TestConsoleNameFor(t *testing.T) {
	cases := []struct {
		container string
		display   string
		want      string
	}{
		{"beta-blood-gulch", "Blood Gulch", "Blood Gulch"}, // pretty wins
		{"beta-play-abc123", "", "beta-play-abc123"},        // fallback to container
		{"smoke", "Stew's Box", "Stew's Box"},
	}
	for _, c := range cases {
		if got := consoleNameFor(c.container, c.display); got != c.want {
			t.Errorf("consoleNameFor(%q,%q) = %q, want %q", c.container, c.display, got, c.want)
		}
	}
}

// TestConsoleNamePrettyRoundTrip: the chosen console name, once Sanitized by the
// writer, preserves the pretty display name's printable-ASCII content (≤15) —
// i.e. spaces/case survive to the Xbox nickname (unlike the slugified container
// name).
func TestConsoleNamePrettyRoundTrip(t *testing.T) {
	display := "Blood Gulch"
	chosen := consoleNameFor("beta-blood-gulch", display)
	if got := consolename.Sanitize(chosen); got != "Blood Gulch" {
		t.Errorf("console name after Sanitize = %q, want %q (pretty display preserved)", got, "Blood Gulch")
	}
	// The container name, by contrast, is the mangled slug — decoupled.
	if consolename.Sanitize("beta-blood-gulch") == display {
		t.Errorf("container slug should NOT equal the pretty display name")
	}
}
