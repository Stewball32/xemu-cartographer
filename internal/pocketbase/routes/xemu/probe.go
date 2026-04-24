package xemu

import (
	"fmt"
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	xemupkg "github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// probeResponse is the diagnostic payload returned by GET /api/admin/xemu/probe.
type probeResponse struct {
	Sock              string `json:"sock"`
	PID               int    `json:"pid"`
	BaseHVA           string `json:"base_hva"`             // hex-formatted uint64
	TitleID           string `json:"title_id"`             // hex-formatted uint32
	DetectError       string `json:"detect_error"`         // expected: "detect: unknown title ID 0x..."
	XBEMagicAt0x10000 string `json:"xbe_magic_at_0x10000"` // expected: 0x48454248 ("HEBX")
}

func init() {
	register(func() {
		// GET /api/admin/xemu/probe?sock=<path>
		//
		// Smoke-test the memory bridge end to end: find the xemu PID, open QMP,
		// translate the base HVA + XBE header, read the title ID, and sample the
		// XBE magic. With no GameReader factories registered (M1), Detect()
		// returns "detect: unknown title ID 0x..." — that's the expected success
		// state for this milestone.
		Group.GET("/probe", func(e *core.RequestEvent) error {
			sock := e.Request.URL.Query().Get("sock")
			if sock == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{
					"error": "sock query parameter is required",
				})
			}
			if _, err := os.Stat(sock); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("sock %q not accessible: %v", sock, err),
				})
			}

			inst := &xemupkg.Instance{Name: "probe", QMPSock: sock}
			if err := inst.Init(scraper.DetectionGVAs()); err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{
					"error": err.Error(),
				})
			}
			defer inst.Close()

			resp := probeResponse{
				Sock:    sock,
				PID:     inst.PID,
				BaseHVA: fmt.Sprintf("0x%016x", inst.Mem.Base()),
			}

			_, titleID, detectErr := scraper.Detect(inst, "probe")
			resp.TitleID = fmt.Sprintf("0x%08X", titleID)
			if detectErr != nil {
				resp.DetectError = detectErr.Error()
			}

			// XBE header lives at low GVA 0x00010000; read via the cached
			// LowHVA translation populated by Init(DetectionGVAs()).
			if headerHVA, err := inst.LowHVA(0x00010000); err != nil {
				resp.XBEMagicAt0x10000 = fmt.Sprintf("low HVA lookup error: %v", err)
			} else if magic, err := inst.Mem.ReadU32At(headerHVA); err != nil {
				resp.XBEMagicAt0x10000 = fmt.Sprintf("read error: %v", err)
			} else {
				resp.XBEMagicAt0x10000 = fmt.Sprintf("0x%08X", magic)
			}

			return e.JSON(http.StatusOK, resp)
		})
	})
}
