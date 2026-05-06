package xemu

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"

	xemupkg "github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// sampleDeltasMatch is one offset whose u32 value changed between two reads.
type sampleDeltasMatch struct {
	GVA   string `json:"gva"`          // u32 LE 4-byte-aligned offset
	V0Hex string `json:"v0_hex"`       // first sample
	V1Hex string `json:"v1_hex"`       // second sample
	Delta int64  `json:"delta"`        // signed v1 - v0 (clamped to int64)
	Rate  int64  `json:"rate_per_sec"` // delta scaled to per-second
}

type sampleDeltasResponse struct {
	Sock       string              `json:"sock"`
	PID        int                 `json:"pid"`
	StartGVA   string              `json:"start_gva"`
	EndGVA     string              `json:"end_gva"`
	IntervalMS int                 `json:"interval_ms"`
	Changed    []sampleDeltasMatch `json:"changed"`
	Truncated  bool                `json:"truncated"`
	Note       string              `json:"note,omitempty"`
}

func init() {
	register(func() {
		// GET /api/admin/xemu/sample-deltas?sock=<path>
		//     &start=0x80050000&end=0x80060000&interval_ms=1000&max=200
		//
		// Investigation tool: read a high-GVA window twice, interval_ms apart,
		// and return every u32-aligned offset whose value changed. Use to find
		// clock-like fields (KeSystemTime delta ≈ 10,000,000/sec FILETIME 100ns
		// ticks; KeTickCount ≈ 100/sec) and other monotonically-incrementing
		// kernel globals.
		Group.GET("/sample-deltas", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			sock := q.Get("sock")
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

			start, err := parseHexU32(q.Get("start"), 0x80050000)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "bad start: " + err.Error()})
			}
			end, err := parseHexU32(q.Get("end"), 0x80060000)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "bad end: " + err.Error()})
			}
			if end <= start {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "end must be > start"})
			}
			// Cap range size — sampling 8 MiB twice is fine but anything bigger
			// burns memory bridge bandwidth without obvious benefit.
			if end-start > 0x00800000 {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "range too large (max 8 MiB)"})
			}
			intervalMS := 1000
			if v, err := strconv.Atoi(q.Get("interval_ms")); err == nil && v >= 50 && v <= 10000 {
				intervalMS = v
			}
			maxMatches := 200
			if v, err := strconv.Atoi(q.Get("max")); err == nil && v > 0 && v <= 2000 {
				maxMatches = v
			}

			inst := &xemupkg.Instance{Name: "sample", QMPSock: sock}
			if err := inst.Init(nil); err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}
			defer inst.Close()

			length := int(end - start)
			buf0, err := inst.Mem.ReadBytes(start, length)
			if err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{"error": "first read: " + err.Error()})
			}
			time.Sleep(time.Duration(intervalMS) * time.Millisecond)
			buf1, err := inst.Mem.ReadBytes(start, length)
			if err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{"error": "second read: " + err.Error()})
			}

			resp := sampleDeltasResponse{
				Sock:       sock,
				PID:        inst.PID,
				StartGVA:   fmt.Sprintf("0x%08X", start),
				EndGVA:     fmt.Sprintf("0x%08X", end),
				IntervalMS: intervalMS,
				Changed:    []sampleDeltasMatch{},
			}

			// Walk u32 4-byte-aligned offsets. For each pair where v0 != v1,
			// emit a match with raw values + delta + per-second rate.
			for i := 0; i+4 <= len(buf0) && i+4 <= len(buf1); i += 4 {
				v0 := binary.LittleEndian.Uint32(buf0[i:])
				v1 := binary.LittleEndian.Uint32(buf1[i:])
				if v0 == v1 {
					continue
				}
				// Signed delta — for monotonic counters this is positive; for
				// fields that wrap or move freely it can be either sign.
				delta := int64(v1) - int64(v0)
				rate := int64(0)
				if intervalMS > 0 {
					rate = delta * 1000 / int64(intervalMS)
				}
				resp.Changed = append(resp.Changed, sampleDeltasMatch{
					GVA:   fmt.Sprintf("0x%08X", start+uint32(i)),
					V0Hex: fmt.Sprintf("0x%08X", v0),
					V1Hex: fmt.Sprintf("0x%08X", v1),
					Delta: delta,
					Rate:  rate,
				})
				if len(resp.Changed) >= maxMatches {
					resp.Truncated = true
					return e.JSON(http.StatusOK, resp)
				}
			}
			if len(resp.Changed) == 0 {
				resp.Note = "no u32 offsets changed in window"
			}
			return e.JSON(http.StatusOK, resp)
		})
	})
}
