package play

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

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

// TestChooseBootFilename covers the pure server/game boot decision: a host
// instance boots the SERVER build when present, else the game's own file.
func TestChooseBootFilename(t *testing.T) {
	cases := []struct {
		name, game, server, want string
	}{
		{"no server build → game file", "halo-ce.iso", "", "halo-ce.iso"},
		{"server build present → server file", "halo-ce.iso", "halo-ce-server.iso", "halo-ce-server.iso"},
		{"whitespace-only server → game file", "halo-ce.iso", "   ", "halo-ce.iso"},
	}
	for _, c := range cases {
		if got := chooseBootFilename(c.game, c.server); got != c.want {
			t.Errorf("%s: chooseBootFilename(%q,%q) = %q, want %q", c.name, c.game, c.server, got, c.want)
		}
	}
}

// TestResolveBootFilename proves the end-to-end host-boot resolution against a
// real catalog: a game with a server_iso boots the SERVER file, a game without
// one boots its OWN file, and DELETING a linked server build makes the game fall
// back to its OWN file (PB nullifies the ref on delete, cascadeDelete=false), so
// removing a server build never bricks the games that referenced it.
func TestResolveBootFilename(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	// Minimal isos catalog: filename + self-referential server_iso relation,
	// mirroring the migration shape without importing the migrations package.
	isos := core.NewBaseCollection("isos")
	isos.Fields.Add(
		&core.TextField{Name: "filename"},
		&core.TextField{Name: "name"},
	)
	if err := app.Save(isos); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	isos, _ = app.FindCollectionByNameOrId("isos")
	isos.Fields.Add(&core.RelationField{Name: "server_iso", CollectionId: isos.Id, MaxSelect: 1})
	if err := app.Save(isos); err != nil {
		t.Fatalf("add server_iso: %v", err)
	}

	newISO := func(name, filename, serverID string) *core.Record {
		r := core.NewRecord(isos)
		r.Set("name", name)
		r.Set("filename", filename)
		if serverID != "" {
			r.Set("server_iso", serverID)
		}
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso %q: %v", name, err)
		}
		return r
	}

	server := newISO("Halo CE (server)", "halo-ce-server.iso", "")
	withServer := newISO("Halo CE", "halo-ce.iso", server.Id)
	noServer := newISO("Halo 2", "halo-2.iso", "")

	if got := resolveBootFilename(app, withServer); got != "halo-ce-server.iso" {
		t.Errorf("server build present: got %q, want halo-ce-server.iso", got)
	}
	if got := resolveBootFilename(app, noServer); got != "halo-2.iso" {
		t.Errorf("no server build: got %q, want halo-2.iso", got)
	}

	// Remove the linked server build; PB nullifies withServer.server_iso, so a
	// fresh read of the game must fall back to its own file — not brick.
	if err := app.Delete(server); err != nil {
		t.Fatalf("delete server iso: %v", err)
	}
	reloaded, err := app.FindRecordById("isos", withServer.Id)
	if err != nil {
		t.Fatalf("reload game: %v", err)
	}
	if got := resolveBootFilename(app, reloaded); got != "halo-ce.iso" {
		t.Errorf("after server delete: got %q, want fallback halo-ce.iso", got)
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
