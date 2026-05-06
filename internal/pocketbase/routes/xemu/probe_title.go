package xemu

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	xemupkg "github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// titleProbeSample is one reading taken by the continuous title-ID probe.
// All raw values are surfaced as hex strings so the response is grep-able.
type titleProbeSample struct {
	Seq          int    `json:"seq"`
	TOffsetMs    int64  `json:"t_offset_ms"`    // milliseconds since the first sample
	TitleID      string `json:"title_id"`       // hex u32
	XBEMagic     string `json:"xbe_magic"`      // hex u32; "XBEH" = 0x48454258
	XBEMagicText string `json:"xbe_magic_text"` // ASCII rendering for quick eyeballing
	ReadError    string `json:"read_error,omitempty"`
}

// titleProbeResponse is the full sample series returned by GET
// /api/admin/xemu/probe-title. Investigation tool for M5 OQ6 — run while
// transitioning Halo CE → quit-to-dashboard and inspect which fields
// change. See ROADMAP.md M5 follow-ups.
type titleProbeResponse struct {
	Sock       string             `json:"sock"`
	PID        int                `json:"pid"`
	BaseHVA    string             `json:"base_hva"`
	IntervalMs int                `json:"interval_ms"`
	Samples    []titleProbeSample `json:"samples"`
}

func init() {
	register(func() {
		// GET /api/admin/xemu/probe-title?sock=<path>&samples=<n>&interval_ms=<ms>
		//
		// Continuously samples the XBE title ID + magic at GVA 0x00010000 and
		// returns the series. Default samples=30, interval_ms=100 → ~3s of
		// readings. Bounded at 600 samples / 5000ms to keep the request budget
		// reasonable.
		//
		// Driving question (M5 OQ6): is the title-ID at 0x00010000 a reliable
		// signal for Halo CE → dashboard transitions, or does xemu leave the
		// stale page mapped after the engine exits? Run this while quitting
		// to the dashboard and observe whether title_id stays 0x4D530004 or
		// flips to a dashboard ID / 0x00000000.
		Group.GET("/probe-title", func(e *core.RequestEvent) error {
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

			samples := parseIntDefault(e.Request.URL.Query().Get("samples"), 30, 1, 600)
			intervalMs := parseIntDefault(e.Request.URL.Query().Get("interval_ms"), 100, 5, 5000)

			inst := &xemupkg.Instance{Name: "probe-title", QMPSock: sock}
			if err := inst.Init(scraper.DetectionGVAs()); err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{
					"error": err.Error(),
				})
			}
			defer inst.Close()

			resp := titleProbeResponse{
				Sock:       sock,
				PID:        inst.PID,
				BaseHVA:    fmt.Sprintf("0x%016x", inst.Mem.Base()),
				IntervalMs: intervalMs,
				Samples:    make([]titleProbeSample, 0, samples),
			}

			start := time.Now()
			for i := 0; i < samples; i++ {
				if i > 0 {
					select {
					case <-e.Request.Context().Done():
						return e.JSON(http.StatusOK, resp)
					case <-time.After(time.Duration(intervalMs) * time.Millisecond):
					}
				}
				s := titleProbeSample{
					Seq:       i,
					TOffsetMs: time.Since(start).Milliseconds(),
				}
				if titleID, err := scraper.ReadTitleID(inst); err != nil {
					s.ReadError = err.Error()
				} else {
					s.TitleID = fmt.Sprintf("0x%08X", titleID)
				}
				if headerHVA, err := inst.LowHVA(0x00010000); err != nil {
					if s.ReadError == "" {
						s.ReadError = fmt.Sprintf("low HVA lookup: %v", err)
					}
				} else if magic, err := inst.Mem.ReadU32At(headerHVA); err != nil {
					if s.ReadError == "" {
						s.ReadError = fmt.Sprintf("magic read: %v", err)
					}
				} else {
					s.XBEMagic = fmt.Sprintf("0x%08X", magic)
					s.XBEMagicText = magicAsASCII(magic)
				}
				resp.Samples = append(resp.Samples, s)
			}

			return e.JSON(http.StatusOK, resp)
		})
	})
}

func parseIntDefault(s string, def, lo, hi int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// magicAsASCII renders the 4 little-endian bytes of magic as a printable
// string (replacing non-printable bytes with '.'). XBEH = "HEBX" when read
// little-endian as Xbox stores the header magic.
func magicAsASCII(magic uint32) string {
	out := []byte{
		byte(magic),
		byte(magic >> 8),
		byte(magic >> 16),
		byte(magic >> 24),
	}
	for i, b := range out {
		if b < 0x20 || b > 0x7E {
			out[i] = '.'
		}
	}
	return string(out)
}
