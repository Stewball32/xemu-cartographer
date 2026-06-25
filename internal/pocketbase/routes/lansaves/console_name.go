package lansaves

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

// Console name — the Xbox dashboard / system-link identity (E:\UDATA\NICKNAME.XBN).
//
//	GET /api/lan/saves/console-name/{gamertag}
//
// Stateless: derived purely from the gamertag (plaintext, no checksum), so it
// needs no stored record. This is the box's console identity — NOT the Halo: CE
// player profile (CE has its own profiles, served via /file/ce-profile/{id} once
// their format lands). Returns a tar with UDATA/NICKNAME.XBN.
func init() {
	register(func() {
		Group.GET("/console-name/{gamertag}", handleConsoleName)
	})
}

func handleConsoleName(e *core.RequestEvent) error {
	gamertag := strings.TrimSpace(e.Request.PathValue("gamertag"))
	if gamertag == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "gamertag is required"})
	}
	b, err := saveartifact.ConsoleNameBundle(gamertag)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	h := e.Response.Header()
	h.Set("Content-Disposition", `attachment; filename="console-name.tar"`)
	h.Set("X-Fatx-Dir", "UDATA")
	return e.Blob(http.StatusOK, "application/x-tar", b.Tar)
}
