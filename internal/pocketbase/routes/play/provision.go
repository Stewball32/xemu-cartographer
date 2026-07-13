package play

import (
	"net/http"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/instancename"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

func init() {
	register(registerISOs)
	register(registerRequest)
}

// isoOption is the player-safe projection of an `isos` catalog row. The raw
// library `filename` is deliberately omitted — the player picks by id, and the
// server resolves the on-disk file when provisioning.
type isoOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	TitleID     string `json:"title_id"`
	Description string `json:"description"`
}

// GET /api/play/isos — the games a player may request an instance for: every
// available (admin-toggled) entry in the ISO library, sorted by name. RequireAuth
// only (group-bound) — any authed player may browse the catalog to pick from; no
// container scoping applies until they actually request one.
func registerISOs() {
	Group.GET("/isos", func(e *core.RequestEvent) error {
		records, err := e.App.FindRecordsByFilter("isos", "available = true", "name", 0, 0, dbx.Params{})
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		out := make([]isoOption, 0, len(records))
		for _, r := range records {
			out = append(out, isoOption{
				ID:          r.Id,
				Name:        r.GetString("name"),
				TitleID:     r.GetString("title_id"),
				Description: r.GetString("description"),
			})
		}
		return e.JSON(http.StatusOK, out)
	})
}

// requestResponse is the payload returned when a player provisions an instance.
type requestResponse struct {
	Instance string `json:"instance"`
	Index    int    `json:"index"`
	ISO      string `json:"iso"`      // the chosen game's display name
	TitleID  string `json:"title_id"` // its title id (may be "")
	GameISO  string `json:"game_iso"` // resolved host path of the attached disc
}

// POST /api/play/request — the player picks a game and gets a box: resolve the
// chosen library ISO, provision + start a fresh xemu instance booting straight
// into it (ADR-0004), and return the new instance. Body:
//
//	{"iso": "<isos record id>", "name": "<optional, admins only>"}
//
// This is the last player-flow piece that was admin-only (routes/containers) —
// it's now self-serve, but narrowly: a non-admin always gets a single stable
// per-user box name ("play-<uid>"), so a player can neither name arbitrary
// containers nor field more than one. Admins may pass an explicit name. Fails
// closed with 409 when that box already exists (tear it down first), 503 when
// provisioning isn't wired (CONTAINERS_ENABLED=false), 403 when the chosen ISO
// isn't player-available. The admin kiosk/VNC path is untouched.
func registerRequest() {
	Group.POST("/request", func(e *core.RequestEvent) error {
		var body struct {
			ISO  string `json:"iso"`
			Name string `json:"name"`
		}
		if err := e.BindBody(&body); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if strings.TrimSpace(body.ISO) == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "iso is required"})
		}

		// Resolve + validate the chosen game BEFORE the provisioning-capability
		// check, so a bad request gets a precise 400/404/403 regardless of whether
		// provisioning happens to be wired. Players may only launch available
		// entries; an admin toggling one off hides it here too.
		rec, err := e.App.FindRecordById("isos", strings.TrimSpace(body.ISO))
		if err != nil {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "game not found in the library"})
		}
		if !rec.GetBool("available") {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "that game is not available to play right now"})
		}
		filename := rec.GetString("filename")
		if filename == "" {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "library entry has no file"})
		}

		// Provisioning subsystem must be wired (CONTAINERS_ENABLED) to go further.
		if Provisioner == nil {
			return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "instance provisioning not enabled"})
		}

		isAdmin := roles.IsAdminAuth(e.App, e.Auth)
		display, name := instanceName(Provisioner.NamePrefix(), e.Auth.Id, body.Name, isAdmin)
		if name == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "could not determine an instance name"})
		}
		if Provisioner.Exists(name) {
			return e.JSON(http.StatusConflict, map[string]string{
				"error": "you already have an instance (" + name + "); tear it down first",
			})
		}

		res, err := Provisioner.Provision(name, filename, display)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return e.JSON(http.StatusCreated, requestResponse{
			Instance: res.Name,
			Index:    res.Index,
			ISO:      rec.GetString("name"),
			TitleID:  rec.GetString("title_id"),
			GameISO:  res.GameISO,
		})
	})
}

// instanceName is the pure DECOUPLED-naming decision for request-instance. It
// returns the canonical pretty display name AND the derived podman container
// name:
//
//   - A non-admin ALWAYS gets a single stable per-user box "play-<uid>" (so a
//     player can't name arbitrary containers or field more than one — the Exists
//     check then fails a second request closed) with an EMPTY display name (the
//     console name falls back to the container name).
//   - An admin may pass an explicit pretty name: it's validated as a display
//     name (printable ASCII, ≤15) and the container name is its slug. If the
//     pretty name slugs to empty (all punctuation), the display is kept but the
//     container falls back to the per-user "play-<uid>" scheme for uniqueness.
//   - prefix (the deployment's container-name namespace, empty in prod) is
//     prepended to the container name so a beta sharing the host podman daemon
//     gives "beta-*" boxes that can't collide with prod's. It is NOT applied to
//     the display name (that's the pretty canonical, unmangled).
//
// container is "" only when there's nothing to derive from (no userID and no
// usable override), so the caller still rejects it. Split out so it's
// unit-testable with no request/podman.
func instanceName(prefix, userID, override string, isAdmin bool) (display, container string) {
	uid := sanitizeName(userID)
	perUser := ""
	if uid != "" {
		perUser = prefix + "play-" + uid
	}
	if isAdmin {
		if d := instancename.Display(override); d != "" {
			if slug := instancename.Slug(d); slug != "" {
				return d, prefix + slug
			}
			// Pretty name present but slugs to empty — keep the display, fall
			// back to the uid scheme for a valid, unique container name.
			return d, perUser
		}
	}
	// Non-admin, or admin with no usable override: uid-based box, no pretty name.
	return "", perUser
}

// sanitizeName lowercases and keeps only podman-safe name characters
// ([a-z0-9_.-]), collapsing everything else away. PB record ids are already
// lowercase alphanumeric, so the derived per-user name passes through intact;
// this only guards an admin's free-form override.
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '.', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}
