package play

import (
	"os"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
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

// TestBootRecordID covers the pure server/game boot decision: a host instance
// boots the SERVER build's record when the game links one, else the game's own.
func TestBootRecordID(t *testing.T) {
	cases := []struct {
		name, game, server, want string
	}{
		{"no server build → game", "game1", "", "game1"},
		{"server build present → server", "game1", "srv1", "srv1"},
		{"whitespace-only server → game", "game1", "   ", "game1"},
	}
	for _, c := range cases {
		if got := bootRecordID(c.game, c.server); got != c.want {
			t.Errorf("%s: bootRecordID(%q,%q) = %q, want %q", c.name, c.game, c.server, got, c.want)
		}
	}
}

// isosTestCollection builds a minimal `isos` collection mirroring the managed
// ingest shape (no PB rules), returning it for record creation.
func isosTestCollection(t *testing.T, app core.App) *core.Collection {
	t.Helper()
	isos := core.NewBaseCollection("isos")
	isos.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "filename"},
		&core.TextField{Name: "content_hash"},
		&core.NumberField{Name: "file_size", OnlyInt: true},
		&core.NumberField{Name: "file_mtime", OnlyInt: true},
		&core.BoolField{Name: "available"},
		&core.BoolField{Name: "drift_detected"},
	)
	if err := app.Save(isos); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	isos, _ = app.FindCollectionByNameOrId("isos")
	isos.Fields.Add(&core.RelationField{Name: "server_iso", CollectionId: isos.Id, MaxSelect: 1})
	if err := app.Save(isos); err != nil {
		t.Fatalf("add server_iso: %v", err)
	}
	return isos
}

// TestResolveBootISO proves the host-boot resolution returns the managed
// <id>.iso for the SERVER build when linked, the game's OWN <id>.iso otherwise,
// and falls back to the game after the server build is deleted (never bricks).
// Rows carry no content_hash here, so the drift check passes trivially.
func TestResolveBootISO(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	isos := isosTestCollection(t, app)

	newISO := func(name, serverID string) *core.Record {
		r := core.NewRecord(isos)
		r.Set("name", name)
		r.Set("available", true)
		if serverID != "" {
			r.Set("server_iso", serverID)
		}
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso %q: %v", name, err)
		}
		return r
	}
	cfg := lansync.Config{}

	server := newISO("Halo CE (server)", "")
	withServer := newISO("Halo CE", server.Id)
	noServer := newISO("Halo 2", "")

	if got, err := resolveBootISO(app, cfg, withServer); err != nil || got != server.Id+".iso" {
		t.Errorf("server build present: got (%q,%v), want %q.iso", got, err, server.Id)
	}
	if got, err := resolveBootISO(app, cfg, noServer); err != nil || got != noServer.Id+".iso" {
		t.Errorf("no server build: got (%q,%v), want %q.iso", got, err, noServer.Id)
	}

	if err := app.Delete(server); err != nil {
		t.Fatalf("delete server: %v", err)
	}
	reloaded, _ := app.FindRecordById("isos", withServer.Id)
	if got, err := resolveBootISO(app, cfg, reloaded); err != nil || got != withServer.Id+".iso" {
		t.Errorf("after server delete: got (%q,%v), want fallback %q.iso", got, err, withServer.Id)
	}
}

// TestResolveBootISO_DriftRefused proves a disc whose managed bytes no longer
// match its content-hash anchor is refused (error) and flagged unavailable —
// bad bytes never boot.
func TestResolveBootISO_DriftRefused(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	isos := isosTestCollection(t, app)

	dir := t.TempDir()
	cfg := lansync.Config{ISODir: dir}

	game := core.NewRecord(isos)
	game.Set("name", "Tampered")
	game.Set("available", true)
	game.Set("content_hash", "0000000000000000000000000000000000000000000000000000000000000000")
	game.Set("file_size", 999999) // wrong → forces a re-hash in VerifyManaged
	if err := app.Save(game); err != nil {
		t.Fatalf("save game: %v", err)
	}
	// Real managed bytes that won't match the stored hash.
	if err := os.WriteFile(cfg.ManagedISOPath(game.Id), []byte("actual bytes"), 0o644); err != nil {
		t.Fatalf("write managed: %v", err)
	}

	if _, err := resolveBootISO(app, cfg, game); err == nil {
		t.Fatal("expected drift refusal, got nil error")
	}
	reloaded, _ := app.FindRecordById("isos", game.Id)
	if reloaded.GetBool("available") || !reloaded.GetBool("drift_detected") {
		t.Errorf("drift row should be available=false + drift_detected=true; got available=%v drift=%v",
			reloaded.GetBool("available"), reloaded.GetBool("drift_detected"))
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
