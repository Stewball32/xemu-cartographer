//go:build dev

package seed

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
	"github.com/Stewball32/xemu-cartographer/internal/isoingest"
	"github.com/Stewball32/xemu-cartographer/internal/lansync"
	hooks "github.com/Stewball32/xemu-cartographer/internal/pocketbase/hooks"
)

// lanTestPlayers are the checked-in players for the E2E LAN-sync scenario. Each
// gets a user (with a gamertag), a gamertags row, and generated CE + H2
// profiles, then a check-in against the test event.
var lanTestPlayers = []struct{ Email, Username, Gamertag string }{
	{"lan1@dev.com", "lanplayer1", "New001"},
	{"lan2@dev.com", "lanplayer2", "New002"},
	{"lan3@dev.com", "lanplayer3", "New003"},
}

// SeedLanSync builds a complete, working LAN-sync scenario so that hitting
// GET /api/lan/sync/manifest?preset=active returns a POPULATED payload and the
// game/app downloads return real files. Idempotent-ish (skips rows that already
// exist by a natural key). Called from Run() (dev builds only).
func SeedLanSync(app *pocketbase.PocketBase) error {
	log.Println("Seeding LAN-sync test scenario...")

	// 1. LAN event (check-in scope).
	event, err := ensureLanEvent(app, "NorCal Test LAN")
	if err != nil {
		return fmt.Errorf("lan_event: %w", err)
	}

	// 2. Players → users + gamertags + CE/H2 profiles + check-ins.
	for _, p := range lanTestPlayers {
		user, err := ensureLanPlayer(app, p.Email, p.Username, p.Gamertag)
		if err != nil {
			return fmt.Errorf("player %s: %w", p.Gamertag, err)
		}
		gt, err := ensureGamertag(app, user.Id, p.Gamertag)
		if err != nil {
			return fmt.Errorf("gamertag %s: %w", p.Gamertag, err)
		}
		if err := ensureProfile(app, "ce_profiles", user.Id); err != nil {
			return fmt.Errorf("ce profile %s: %w", p.Gamertag, err)
		}
		if err := ensureProfile(app, "h2_profiles", user.Id); err != nil {
			return fmt.Errorf("h2 profile %s: %w", p.Gamertag, err)
		}
		if err := ensureCheckin(app, event.Id, gt.Id); err != nil {
			return fmt.Errorf("checkin %s: %w", p.Gamertag, err)
		}
	}

	// 3. A real, extractable game ISO in the bank (placeholder XISO created via
	//    extract-xiso -c, then extracted via the real hook path).
	iso, err := ensureTestISO(app)
	if err != nil {
		return fmt.Errorf("test iso: %w", err)
	}

	// 4. A real app zip.
	appRec, err := ensureTestApp(app)
	if err != nil {
		return fmt.Errorf("test app: %w", err)
	}

	// 5. The ACTIVE preset selecting them with priority + policy.
	if err := ensureActivePreset(app, event.Id, iso.Id, appRec.Id); err != nil {
		return fmt.Errorf("preset: %w", err)
	}

	log.Println("LAN-sync test scenario seeded.")
	return nil
}

func ensureLanEvent(app *pocketbase.PocketBase, label string) (*core.Record, error) {
	if r, _ := app.FindFirstRecordByFilter("lan_events", "label = {:l}", map[string]any{"l": label}); r != nil {
		return r, nil
	}
	col, err := app.FindCollectionByNameOrId("lan_events")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("label", label)
	r.Set("description", "E2E LAN-sync test session")
	r.Set("active", true)
	return r, app.Save(r)
}

func ensureLanPlayer(app *pocketbase.PocketBase, email, username, gamertag string) (*core.Record, error) {
	if existing, _ := app.FindAuthRecordByEmail("users", email); existing != nil {
		return existing, nil
	}
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("email", email)
	r.Set("password", "lanpass123")
	r.Set("username", username)
	r.Set("gamertag", gamertag) // read by the profile generate hooks
	if err := app.Save(r); err != nil {
		return nil, err
	}
	return r, nil
}

func ensureGamertag(app *pocketbase.PocketBase, userID, tag string) (*core.Record, error) {
	if r, _ := app.FindFirstRecordByFilter("gamertags",
		"user = {:u} && tag = {:t}", map[string]any{"u": userID, "t": tag}); r != nil {
		return r, nil
	}
	col, err := app.FindCollectionByNameOrId("gamertags")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("user", userID)
	r.Set("tag", tag) // sanitize hook fills `sanitized`
	r.Set("status", "approved")
	return r, app.Save(r)
}

// ensureProfile creates a ce_profiles / h2_profiles row for a user; the
// generate-on-save hook signs the bundle into save_bundle + save_info.
func ensureProfile(app *pocketbase.PocketBase, collection, userID string) error {
	if r, _ := app.FindFirstRecordByFilter(collection, "user = {:u}", map[string]any{"u": userID}); r != nil {
		return nil
	}
	col, err := app.FindCollectionByNameOrId(collection)
	if err != nil {
		return err
	}
	r := core.NewRecord(col)
	r.Set("user", userID)
	return app.Save(r) // OnRecordCreate generate hook attaches the bundle
}

func ensureCheckin(app *pocketbase.PocketBase, eventID, gamertagID string) error {
	if r, _ := app.FindFirstRecordByFilter("checkins",
		"event = {:e} && gamertag = {:g}", map[string]any{"e": eventID, "g": gamertagID}); r != nil {
		return nil
	}
	col, err := app.FindCollectionByNameOrId("checkins")
	if err != nil {
		return err
	}
	r := core.NewRecord(col)
	r.Set("event", eventID)
	r.Set("gamertag", gamertagID)
	r.Set("source", "organizer")
	return app.Save(r)
}

// ensureTestISO drops a small, VALID XISO into the ISO bank (built with
// extract-xiso -c from a scratch dir) and catalogs it, then extracts it through
// the real hook path so extracted_ready + footprint_bytes are set.
func ensureTestISO(app *pocketbase.PocketBase) (*core.Record, error) {
	const filename = "test-halo.iso"
	cfg := lansync.Load()

	if r, _ := app.FindFirstRecordByFilter("isos", "filename = {:f}", map[string]any{"f": filename}); r != nil {
		return r, nil
	}

	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}

	col, err := app.FindCollectionByNameOrId("isos")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("name", "Halo: Combat Evolved (test)")
	r.Set("filename", filename) // provenance only — managed file is <id>.iso
	r.Set("title_id", halosave.TitleIDHaloCE)
	r.Set("dest_name", "HaloCE")
	r.Set("role", "play")
	r.Set("allow_on_xbox", true)
	if err := app.Save(r); err != nil {
		return nil, err
	}
	// Build the placeholder XISO directly at the managed <id>.iso path (the new
	// ingest model), hash it as the drift anchor, then extract synchronously so
	// the manifest/download see it ready immediately.
	managed := cfg.ManagedISOPath(r.Id)
	if _, statErr := os.Stat(managed); statErr != nil {
		if err := buildPlaceholderXISO(cfg.ExtractXISOCmd, managed); err != nil {
			return nil, err
		}
	}
	if h, err := lansync.HashFile(managed); err == nil {
		size, mtime, _ := lansync.StatSig(managed)
		r.Set("content_hash", h)
		r.Set("file_size", size)
		r.Set("file_mtime", mtime)
		if err := app.Save(r); err != nil {
			return nil, err
		}
	}
	if err := isoingest.Extract(app, cfg, r.Id); err != nil {
		return nil, fmt.Errorf("extract iso: %w", err)
	}
	return r, nil
}

// buildPlaceholderXISO writes a scratch dir with a couple files and packs it
// into a real XISO with `extract-xiso -c`, so the extraction hook has a valid
// disc to expand.
func buildPlaceholderXISO(extractXISOCmd, isoPath string) error {
	src, err := os.MkdirTemp("", "xiso-src-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(src)
	if err := os.WriteFile(filepath.Join(src, "default.xbe"), bytes.Repeat([]byte("XBE\x00"), 4096), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(src, "readme.txt"), []byte("xemu-cartographer LAN-sync test disc\n"), 0o644); err != nil {
		return err
	}
	if extractXISOCmd == "" {
		extractXISOCmd = "extract-xiso"
	}
	// extract-xiso -c <dir> <name> — create the xiso at the given path.
	cmd := exec.Command(extractXISOCmd, "-c", src, isoPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract-xiso -c: %w: %s", err, string(out))
	}
	return nil
}

// ensureTestApp uploads a small placeholder app zip + measures it via the hook.
func ensureTestApp(app *pocketbase.PocketBase) (*core.Record, error) {
	if r, _ := app.FindFirstRecordByFilter("apps", "name = {:n}", map[string]any{"n": "XBMC4Gamers (test)"}); r != nil {
		return r, nil
	}
	zipBytes, err := buildPlaceholderZip()
	if err != nil {
		return nil, err
	}
	f, err := filesystem.NewFileFromBytes(zipBytes, "xbmc4gamers-test.zip")
	if err != nil {
		return nil, err
	}
	col, err := app.FindCollectionByNameOrId("apps")
	if err != nil {
		return nil, err
	}
	r := core.NewRecord(col)
	r.Set("name", "XBMC4Gamers (test)")
	r.Set("description", "Placeholder app for the LAN-sync E2E test")
	r.Set("dest_name", "XBMC4Gamers")
	r.Set("available", true)
	r.Set("file", []*filesystem.File{f})
	if err := app.Save(r); err != nil {
		return nil, err
	}
	if err := hooks.ExtractAppRecord(app, r.Id); err != nil {
		return nil, fmt.Errorf("measure app: %w", err)
	}
	return r, nil
}

// buildPlaceholderZip returns a small in-memory zip with a couple of files.
func buildPlaceholderZip() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"default.xbe":      "placeholder xbe payload",
		"skins/readme.txt": "xemu-cartographer LAN-sync test app\n",
	} {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ensureActivePreset creates the ACTIVE preset selecting the test game + app
// with priorities + a per-category policy.
func ensureActivePreset(app *pocketbase.PocketBase, eventID, isoID, appID string) error {
	if r, _ := app.FindFirstRecordByFilter("sync_presets", "name = {:n}", map[string]any{"n": "Saturday Main"}); r != nil {
		return nil
	}
	col, err := app.FindCollectionByNameOrId("sync_presets")
	if err != nil {
		return err
	}
	r := core.NewRecord(col)
	r.Set("name", "Saturday Main")
	r.Set("active", true)
	r.Set("event", eventID)
	r.Set("games", []string{isoID})
	r.Set("apps", []string{appID})
	r.Set("priority", map[string]int{isoID: 100, appID: 50})
	r.Set("policy", map[string]any{
		"profiles": map[string]any{"conflict": "overwrite", "prune": true},
		"games":    map[string]any{"conflict": "skip", "prune": false},
		"apps":     map[string]any{"conflict": "skip", "prune": false},
	})
	return app.Save(r)
}
