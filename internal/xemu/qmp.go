package xemu

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// qmpClient holds an open, handshaked QMP connection.
type qmpClient struct {
	conn    net.Conn
	scanner *bufio.Scanner
}

func newQMPClient(sockPath string) (*qmpClient, error) {
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connect %s: %w", sockPath, err)
	}
	c := &qmpClient{conn: conn, scanner: bufio.NewScanner(conn)}
	// Read greeting banner.
	if !c.scanner.Scan() {
		_ = conn.Close()
		return nil, fmt.Errorf("no QMP banner from %s", sockPath)
	}
	// Negotiate capabilities (required before any command).
	if _, err := fmt.Fprintln(conn, `{"execute":"qmp_capabilities"}`); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send qmp_capabilities to %s: %w", sockPath, err)
	}
	if !c.scanner.Scan() {
		_ = conn.Close()
		return nil, fmt.Errorf("no capabilities response from %s", sockPath)
	}
	return c, nil
}

func (c *qmpClient) close() { _ = c.conn.Close() }

// hmp sends a Human Monitor Protocol command and returns the trimmed return string.
func (c *qmpClient) hmp(cmd string) (string, error) {
	req := fmt.Sprintf(`{"execute":"human-monitor-command","arguments":{"command-line":%q}}`, cmd)
	if _, err := fmt.Fprintln(c.conn, req); err != nil {
		return "", fmt.Errorf("send %q: %w", cmd, err)
	}
	if !c.scanner.Scan() {
		return "", fmt.Errorf("no response for %q", cmd)
	}
	var resp struct{ Return string }
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse response for %q: %w", cmd, err)
	}
	return strings.TrimSpace(resp.Return), nil
}

// sendKeyRaw issues an already-formatted HMP `sendkey` command line and maps
// the result to an error. HMP `sendkey` returns an empty string on success;
// any non-empty return (e.g. "unknown key: 'foo'") is surfaced as an error so
// callers don't silently believe a rejected keypress landed.
func (c *qmpClient) sendKeyRaw(cmd string) error {
	ret, err := c.hmp(cmd)
	if err != nil {
		return err
	}
	if s := strings.TrimSpace(ret); s != "" {
		return fmt.Errorf("%q rejected: %s", cmd, s)
	}
	return nil
}

// ErrNotMapped reports that the guest has no translation for an address YET —
// the monitor answered with prose ("Unmapped", "No memory is mapped at …")
// instead of an address. It is a NOT-READY signal, not a failure: during the
// first seconds of a container's boot the guest page tables aren't built, so
// every translation legitimately answers this way. Callers that can wait should
// retry (see Instance.InitWait); callers that can't should keep whatever
// mapping they already had rather than caching a bad one.
//
// Test with errors.Is(err, ErrNotMapped) — it is wrapped, never returned bare,
// so the raw monitor text stays in the message for diagnosis.
var ErrNotMapped = errors.New("guest memory not mapped yet")

// parseHexSuffix extracts the last whitespace-separated token and parses it as a
// hex uint64.
//
// A translate command answers with an address or with prose. Anything
// non-numeric therefore means "no translation available", which is
// ErrNotMapped — NOT a parse bug. Reporting it as a raw strconv error is what
// crashed scraper attach with
//
//	translate 0x10000: gva2gpa 0x10000: parse "Unmapped": strconv.ParseUint: …
//
// when the scraper auto-attached ~6s into boot. We classify by "did the monitor
// give us a number" rather than matching known phrases, so a new/reworded qemu
// message degrades to a retry instead of a hard failure.
func parseHexSuffix(s string) (uint64, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty response: %q", s)
	}
	// Phrase check BEFORE the numeric one: gpa2hva's failure reply is
	// "No memory is mapped at address 0x10000" — it ENDS IN A VALID NUMBER, so
	// a trailing-token parse would happily accept the queried address as if it
	// were a successful translation and then read garbage from it.
	if isNotMappedResponse(s) {
		return 0, fmt.Errorf("monitor returned %q: %w", strings.TrimSpace(s), ErrNotMapped)
	}
	v, err := strconv.ParseUint(fields[len(fields)-1], 0, 64)
	if err != nil {
		// No number at all (e.g. bare "Unmapped"), and no phrase we recognise:
		// still "the monitor gave us no address", so treat it as not-ready and
		// let the caller retry rather than crashing on a strconv error.
		return 0, fmt.Errorf("monitor returned %q: %w", strings.TrimSpace(s), ErrNotMapped)
	}
	return v, nil
}

// isNotMappedResponse matches the monitor's "there is no translation" replies.
// Kept as an explicit list because these can carry a trailing address that would
// otherwise parse as a valid result; the numeric fallback in parseHexSuffix
// covers any wording not listed here.
func isNotMappedResponse(s string) bool {
	l := strings.ToLower(s)
	for _, phrase := range []string{
		"unmapped",
		"no memory is mapped",
		"cannot access",
		"bad address",
	} {
		if strings.Contains(l, phrase) {
			return true
		}
	}
	return false
}

// gpa2hva translates a guest physical address to a host virtual address.
// Response: "Host virtual address for 0x... (...) is 0x..."
func (c *qmpClient) gpa2hva(gpa uint64) (uint64, error) {
	ret, err := c.hmp(fmt.Sprintf("gpa2hva 0x%x", gpa))
	if err != nil {
		return 0, err
	}
	return parseHexSuffix(ret)
}

// gva2gpa translates a guest virtual address to a guest physical address.
// Response: "Physical address for 0x... is 0x..."
func (c *qmpClient) gva2gpa(gva uint32) (uint64, error) {
	ret, err := c.hmp(fmt.Sprintf("gva2gpa 0x%x", gva))
	if err != nil {
		return 0, err
	}
	return parseHexSuffix(ret)
}

// translateLowGVA translates a guest VA < 0x80000000 to a host VA via gva2gpa + gpa2hva.
func (c *qmpClient) translateLowGVA(gva uint32) (int64, error) {
	gpa, err := c.gva2gpa(gva)
	if err != nil {
		return 0, fmt.Errorf("gva2gpa 0x%x: %w", gva, err)
	}
	hva, err := c.gpa2hva(gpa)
	if err != nil {
		return 0, fmt.Errorf("gpa2hva 0x%x: %w", gpa, err)
	}
	return int64(hva), nil
}
