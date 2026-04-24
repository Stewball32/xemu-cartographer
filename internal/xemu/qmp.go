package xemu

import (
	"bufio"
	"encoding/json"
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
		conn.Close()
		return nil, fmt.Errorf("no QMP banner from %s", sockPath)
	}
	// Negotiate capabilities (required before any command).
	fmt.Fprintln(conn, `{"execute":"qmp_capabilities"}`)
	if !c.scanner.Scan() {
		conn.Close()
		return nil, fmt.Errorf("no capabilities response from %s", sockPath)
	}
	return c, nil
}

func (c *qmpClient) close() { c.conn.Close() }

// hmp sends a Human Monitor Protocol command and returns the trimmed return string.
func (c *qmpClient) hmp(cmd string) (string, error) {
	req := fmt.Sprintf(`{"execute":"human-monitor-command","arguments":{"command-line":%q}}`, cmd)
	fmt.Fprintln(c.conn, req)
	if !c.scanner.Scan() {
		return "", fmt.Errorf("no response for %q", cmd)
	}
	var resp struct{ Return string }
	if err := json.Unmarshal(c.scanner.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("parse response for %q: %w", cmd, err)
	}
	return strings.TrimSpace(resp.Return), nil
}

// parseHexSuffix extracts the last whitespace-separated token and parses it as a hex uint64.
func parseHexSuffix(s string) (uint64, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty response: %q", s)
	}
	v, err := strconv.ParseUint(fields[len(fields)-1], 0, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", fields[len(fields)-1], err)
	}
	return v, nil
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
