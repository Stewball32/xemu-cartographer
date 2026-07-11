package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
)

// qmpMemReader reads guest memory via QMP `memsave` (HMP human-monitor-command)
// instead of /proc/<pid>/mem. memsave writes a file on the host where xemu runs;
// for a CONTAINERISED xemu that host is inside the container, so the caller
// supplies the container-side dir xemu writes to (writeDir, e.g. /qmp) and the
// host-side dir that bind-maps to it (readDir, e.g. containers/xemu/qmp). For a
// native host rig the two are equal.
//
// Unlike the /proc reader (internal/xemu), this path needs no ptrace access, so
// it works as an UNPRIVILEGED user against a rootless container — where YAMA
// ptrace_scope blocks a non-ancestor from opening /proc/<pid>/mem even at the
// same uid. Production reads /proc as root; this is the harness/diagnostic path.
type qmpMemReader struct {
	conn     net.Conn
	r        *bufio.Reader
	writeDir string
	readDir  string
}

func dialQMP(sock, writeDir, readDir string) (*qmpMemReader, error) {
	c, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}
	q := &qmpMemReader{conn: c, r: bufio.NewReader(c), writeDir: writeDir, readDir: readDir}
	if _, err := q.r.ReadBytes('\n'); err != nil { // greeting
		return nil, err
	}
	if _, err := q.cmd(`{"execute":"qmp_capabilities"}`); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *qmpMemReader) cmd(js string) (map[string]any, error) {
	if _, err := q.conn.Write([]byte(js + "\r\n")); err != nil {
		return nil, err
	}
	for {
		line, err := q.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			continue
		}
		if _, ok := m["event"]; ok { // skip async events
			continue
		}
		return m, nil
	}
}

func (q *qmpMemReader) hmp(line string) error {
	esc, _ := json.Marshal(line)
	_, err := q.cmd(fmt.Sprintf(`{"execute":"human-monitor-command","arguments":{"command-line":%s}}`, esc))
	return err
}

// readN memsaves n bytes at a guest-virtual address and returns them.
func (q *qmpMemReader) readN(gva uint32, n int) ([]byte, error) {
	base := fmt.Sprintf("hrp_%x.bin", gva)
	cpath := filepath.Join(q.writeDir, base) // where xemu writes (container view)
	hpath := filepath.Join(q.readDir, base)  // where we read (host view)
	_ = os.Remove(hpath)
	if err := q.hmp(fmt.Sprintf(`memsave 0x%X %d "%s"`, gva, n, cpath)); err != nil {
		return nil, err
	}
	for i := 0; i < 50; i++ {
		if fi, err := os.Stat(hpath); err == nil && fi.Size() >= int64(n) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, err := os.ReadFile(hpath)
	if err != nil {
		return nil, err
	}
	_ = os.Remove(hpath)
	if len(b) < n {
		return nil, fmt.Errorf("short memsave read %d < %d", len(b), n)
	}
	return b[:n], nil
}

func (q *qmpMemReader) close() { _ = q.conn.Close() }

// readObservationQMP reads game_connection + main_menu via QMP memsave and
// projects them into a hostrunner.Observation. Lobby fields (map/gametype/
// machines) need the struct-pointer reads the /proc reader does, so they are
// left empty here — enough to classify main_menu / system_link / hosting and
// gate a single press, which is all the probe's first-live milestone needs.
func readObservationQMP(sock, writeDir, readDir string) hostrunner.Observation {
	q, err := dialQMP(sock, writeDir, readDir)
	if err != nil {
		log.Fatalf("QMP dial %s: %v", sock, err)
	}
	defer q.close()

	gcb, err := q.readN(haloce.AddrGameConnection, 4)
	if err != nil {
		log.Fatalf("read game_connection via memsave: %v", err)
	}
	mmb, err := q.readN(haloce.AddrMainMenuActive, 4)
	if err != nil {
		log.Fatalf("read main_menu via memsave: %v", err)
	}

	ro := hostrunner.ScraperReadout{
		Fresh:          true,
		GameState:      "menu",
		MenuActive:     mmb[0] != 0,
		GameConnection: int(binary.LittleEndian.Uint16(gcb)),
	}
	return ro.Observation()
}
