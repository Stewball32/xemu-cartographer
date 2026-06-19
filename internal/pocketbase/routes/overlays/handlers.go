package overlays

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/overlaytoken"
)

func init() {
	register(registerMint)
	register(registerList)
	register(registerRevoke)
}

type mintRequest struct {
	Room       string `json:"room"`
	Label      string `json:"label"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type mintResponse struct {
	Token     string `json:"token"`
	Kid       string `json:"kid"`
	Room      string `json:"room"`
	ExpiresAt string `json:"expires_at"`
	// URL is the overlay page URL (render page deferred — the path shape is
	// stable so "Copy overlay URL" works today); WSURL is the raw WS endpoint.
	URL   string `json:"url"`
	WSURL string `json:"ws_url"`
}

func registerMint() {
	Group.POST("", func(e *core.RequestEvent) error {
		if !canManageOverlays(e.App, e.Auth) {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "missing overlay-manage permission"})
		}
		var req mintRequest
		if err := e.BindBody(&req); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
		}
		instance, ok := validateRoom(req.Room)
		if !ok {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "room must be host:<instance> (not a summary or class room)"})
		}

		ttl := DefaultTTL
		if req.TTLSeconds > 0 {
			ttl = time.Duration(req.TTLSeconds) * time.Second
		}

		m, err := overlaytoken.Mint(e.App, req.Room, req.Label, ttl, e.Auth, time.Now())
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "mint failed"})
		}
		return e.JSON(http.StatusCreated, mintResponse{
			Token:     m.Token,
			Kid:       m.Kid,
			Room:      m.Room,
			ExpiresAt: m.ExpiresAt.Format(time.RFC3339),
			URL:       "/overlays/" + instance + "/?token=" + m.Token,
			WSURL:     "/api/ws?token=" + m.Token,
		})
	})
}

type tokenInfo struct {
	Kid       string `json:"kid"`
	Room      string `json:"room"`
	Label     string `json:"label"`
	ExpiresAt string `json:"expires_at"`
	Created   string `json:"created"`
	MintedBy  string `json:"minted_by"`
}

func registerList() {
	Group.GET("", func(e *core.RequestEvent) error {
		if !canManageOverlays(e.App, e.Auth) {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "missing overlay-manage permission"})
		}
		// Active (non-revoked) tokens, newest first. Admins + overlay_managers
		// see all for v1; scoping the list per-minter is a follow-up.
		rows, err := e.App.FindRecordsByFilter("overlay_tokens", "revoked = false", "-created", 0, 0, dbx.Params{})
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "list failed"})
		}
		out := make([]tokenInfo, 0, len(rows))
		for _, r := range rows {
			out = append(out, tokenInfo{
				Kid:       r.GetString("kid"),
				Room:      r.GetString("room"),
				Label:     r.GetString("label"),
				ExpiresAt: r.GetDateTime("expires_at").String(),
				Created:   r.GetDateTime("created").String(),
				MintedBy:  r.GetString("minted_by"),
			})
		}
		return e.JSON(http.StatusOK, map[string]any{"tokens": out})
	})
}

func registerRevoke() {
	Group.POST("/{kid}/revoke", func(e *core.RequestEvent) error {
		if !canManageOverlays(e.App, e.Auth) {
			return e.JSON(http.StatusForbidden, map[string]string{"error": "missing overlay-manage permission"})
		}
		kid := e.Request.PathValue("kid")
		if kid == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "kid required"})
		}
		if err := overlaytoken.Revoke(e.App, kid, e.Auth); err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "revoke failed"})
		}
		return e.NoContent(http.StatusNoContent)
	})
}

// validateRoom accepts a "host:<instance>" room and returns the instance. It
// rejects class sub-rooms (host:x:game), the reserved aggregate feeds
// (summary/all), and anything not host-prefixed — an overlay token binds to one
// per-instance room.
func validateRoom(room string) (instance string, ok bool) {
	const prefix = "host:"
	if !strings.HasPrefix(room, prefix) {
		return "", false
	}
	inst := strings.TrimPrefix(room, prefix)
	if inst == "" || strings.ContainsRune(inst, ':') || inst == "summary" || inst == "all" {
		return "", false
	}
	return inst, true
}
