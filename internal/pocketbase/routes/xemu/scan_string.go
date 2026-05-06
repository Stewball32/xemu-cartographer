package xemu

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/xbox"
	xemupkg "github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// scanMatch is one hit in a scan-string response.
type scanMatch struct {
	GVA     string `json:"gva"`     // hex-formatted uint32
	Context string `json:"context"` // ~64 bytes hex around the match
}

type scanResponse struct {
	Sock      string      `json:"sock"`
	PID       int         `json:"pid"`
	Encoding  string      `json:"encoding"`
	Query     string      `json:"query"`
	StartGVA  string      `json:"start_gva"`
	EndGVA    string      `json:"end_gva"`
	BytesRead uint64      `json:"bytes_read"`
	Matches   []scanMatch `json:"matches"`
	Truncated bool        `json:"truncated"`
	Note      string      `json:"note,omitempty"`
}

// parseHexU32 accepts "0x..." or bare hex.
func parseHexU32(s string, fallback uint32) (uint32, error) {
	if s == "" {
		return fallback, nil
	}
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		s = s[2:]
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func init() {
	register(func() {
		// GET /api/admin/xemu/scan-string?sock=<path>&q=<text>
		//     &start=0x81000000&end=0x90000000&encoding=utf16le&max=20
		//
		// Investigation tool: scan a high-GVA window of the xemu process for
		// occurrences of a string. Useful for locating where dashboard /
		// userdata strings live in memory before there's a GameReader bound.
		Group.GET("/scan-string", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			sock := q.Get("sock")
			needle := q.Get("q")
			if sock == "" || needle == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{
					"error": "sock and q query parameters are required",
				})
			}
			if _, err := os.Stat(sock); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("sock %q not accessible: %v", sock, err),
				})
			}

			start, err := parseHexU32(q.Get("start"), 0x81000000)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "bad start: " + err.Error()})
			}
			end, err := parseHexU32(q.Get("end"), 0x90000000)
			if err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "bad end: " + err.Error()})
			}
			if end <= start {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "end must be > start"})
			}
			maxMatches := 20
			if v, err := strconv.Atoi(q.Get("max")); err == nil && v > 0 && v <= 200 {
				maxMatches = v
			}
			contextBytes := 16
			if v, err := strconv.Atoi(q.Get("context")); err == nil && v >= 0 && v <= 512 {
				contextBytes = v
			}
			encoding := q.Get("encoding")
			if encoding == "" {
				encoding = "utf16le"
			}

			var pattern []byte
			switch encoding {
			case "utf16le":
				pattern = xbox.EncodeUTF16LE(needle)
			case "ascii", "utf8":
				pattern = []byte(needle)
			case "hex":
				// Raw byte sequence — accepts "0x..." prefix, optional whitespace
				// or colons, MSB-first. Useful for pointer-scans (encode a u32
				// LE manually as e.g. "78 56 34 12") and for known-byte greps
				// like a 3-byte MAC OUI ("0x0050F2").
				cleaned := strings.NewReplacer(" ", "", ":", "", "0x", "", "0X", "").Replace(needle)
				raw, err := hex.DecodeString(cleaned)
				if err != nil {
					return e.JSON(http.StatusBadRequest, map[string]string{"error": "encoding=hex needs hex bytes (got " + needle + "): " + err.Error()})
				}
				pattern = raw
			default:
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "encoding must be utf16le, utf8, ascii, or hex"})
			}
			if len(pattern) == 0 {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "empty pattern"})
			}

			// Init only needs the base HVA — pass empty low-GVA list.
			inst := &xemupkg.Instance{Name: "scan", QMPSock: sock}
			if err := inst.Init(nil); err != nil {
				return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}
			defer inst.Close()

			resp := scanResponse{
				Sock:     sock,
				PID:      inst.PID,
				Encoding: encoding,
				Query:    needle,
				StartGVA: fmt.Sprintf("0x%08X", start),
				EndGVA:   fmt.Sprintf("0x%08X", end),
				Matches:  []scanMatch{},
			}

			const chunk = 1 << 20 // 1 MiB
			overlap := len(pattern) - 1
			var prevTail []byte
			gva := start
			for gva < end {
				readN := uint32(chunk)
				if gva+readN > end {
					readN = end - gva
				}
				buf, err := inst.Mem.ReadBytes(gva, int(readN))
				if err != nil {
					gva += readN
					prevTail = nil
					continue
				}
				resp.BytesRead += uint64(len(buf))

				// Stitch a small tail from the previous chunk so a match
				// straddling the boundary still hits.
				search := buf
				baseGVA := gva
				if len(prevTail) > 0 {
					stitched := make([]byte, len(prevTail)+len(buf))
					copy(stitched, prevTail)
					copy(stitched[len(prevTail):], buf)
					search = stitched
					baseGVA = gva - uint32(len(prevTail))
				}

				offset := 0
				for {
					i := bytes.Index(search[offset:], pattern)
					if i < 0 {
						break
					}
					hit := offset + i
					hitGVA := baseGVA + uint32(hit)
					ctxStart := hit - contextBytes
					if ctxStart < 0 {
						ctxStart = 0
					}
					ctxEnd := hit + len(pattern) + contextBytes
					if ctxEnd > len(search) {
						ctxEnd = len(search)
					}
					resp.Matches = append(resp.Matches, scanMatch{
						GVA:     fmt.Sprintf("0x%08X", hitGVA),
						Context: fmt.Sprintf("% x", search[ctxStart:ctxEnd]),
					})
					if len(resp.Matches) >= maxMatches {
						resp.Truncated = true
						return e.JSON(http.StatusOK, resp)
					}
					offset = hit + len(pattern)
				}

				if overlap > 0 && len(buf) >= overlap {
					prevTail = append([]byte(nil), buf[len(buf)-overlap:]...)
				} else {
					prevTail = nil
				}
				gva += readN
			}
			if len(resp.Matches) == 0 {
				resp.Note = "no matches in window"
			}
			return e.JSON(http.StatusOK, resp)
		})
	})
}
