// Command widgetprobe is a throwaway in-CONTAINER verification of the CE menu-
// widget cursor read: it points the SAME cartographer scraper code (xemu.Instance
// + haloce.Reader) at a running xemu's QMP socket, reads game state + the SELECT
// MAP / SELECT GAMETYPE +0x4C cursor, and prints them. Run it inside the xemu
// container (as root, so it can read the non-dumpable xemu's /proc/pid/mem — the
// same privilege the production server has) to prove the gpa2hva(0)+/proc read
// path works in-container, not just on the native rig.
//
//	widgetprobe <qmp-sock> [iterations] [sleep-ms]
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "nav" {
		navMode()
		return
	}
	if len(os.Args) >= 3 && os.Args[1] == "enum" {
		enumMode(os.Args[2])
		return
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: widgetprobe <qmp-sock> [iterations] [sleep-ms]")
		fmt.Fprintln(os.Stderr, "       widgetprobe nav <qmp-sock> <ws-url> <map|gt> <target>")
		os.Exit(2)
	}
	sock := os.Args[1]
	iters := 1
	if len(os.Args) > 2 {
		iters, _ = strconv.Atoi(os.Args[2])
	}
	sleepMs := 500
	if len(os.Args) > 3 {
		sleepMs, _ = strconv.Atoi(os.Args[3])
	}

	inst := &xemu.Instance{Name: "probe", QMPSock: sock}
	if err := inst.Init(haloce.AllLowGVAs); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	defer inst.Close()
	fmt.Printf("pid=%d base=0x%x  (proc/mem read path OK)\n", inst.PID, inst.Mem.Base())

	r := haloce.NewReader(inst, "probe", haloce.BaselineOffsets())
	cur, _ := any(r).(scraper.LobbyCursorReader)
	for i := 0; i < iters; i++ {
		state, tick, err := r.ReadGameState()
		mm := r.LastStateInputs()["main_menu"]
		gc := r.LastStateInputs()["game_connection"]
		line := fmt.Sprintf("[%d] state=%s tick=%d main_menu=%v game_conn=%v", i, state, tick, mm, gc)
		if err != nil {
			line += fmt.Sprintf(" (readErr=%v)", err)
		}
		if cur != nil {
			c := cur.ReadLobbyCursor()
			line += fmt.Sprintf("  MAP{idx=%d cnt=%d valid=%v} GT{idx=%d cnt=%d valid=%v}",
				c.MapIndex, c.MapCount, c.MapValid, c.GametypeIndex, c.GametypeCount, c.GametypeValid)
		}
		fmt.Println(line)
		if i < iters-1 {
			time.Sleep(time.Duration(sleepMs) * time.Millisecond)
		}
	}
}

// navMode drives the create-game carousel to a target index CLOSED-LOOP, entirely
// in-container: it reads the live +0x4C cursor (haloce.ReadLobbyCursor, gpa2hva+
// /proc) AND taps Right/Left via the PRODUCTION vncinput.Injector on the browser
// websockify (reachable over the shared host network) — the same read + input
// paths the deployed scraper uses. Presses toward the target the shorter way
// around the wrapping ring, re-reading each step, and stops when cursor == target.
//
//	widgetprobe nav <qmp-sock> <ws-url> <map|gt> <target>
func navMode() {
	if len(os.Args) < 6 {
		fmt.Fprintln(os.Stderr, "usage: widgetprobe nav <qmp-sock> <ws-url> <map|gt> <target>")
		os.Exit(2)
	}
	sock, wsURL, list := os.Args[2], os.Args[3], os.Args[4]
	target, _ := strconv.Atoi(os.Args[5])

	inst := &xemu.Instance{Name: "nav", QMPSock: sock}
	if err := inst.Init(haloce.AllLowGVAs); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	defer inst.Close()
	r := haloce.NewReader(inst, "nav", haloce.BaselineOffsets())

	// read the SETTLED cursor for the chosen list (retry past scroll-animation garbage).
	read := func() (idx, cnt int, ok bool) {
		for i := 0; i < 40; i++ {
			c := r.ReadLobbyCursor()
			if list == "gt" || list == "gametype" {
				if c.GametypeValid {
					return c.GametypeIndex, c.GametypeCount, true
				}
			} else if c.MapValid {
				return c.MapIndex, c.MapCount, true
			}
			time.Sleep(60 * time.Millisecond)
		}
		return 0, 0, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	inj, err := vncinput.Dial(ctx, wsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer inj.Close()
	_ = inj.FocusClick() // production pump focuses the Selkies canvas on connect
	time.Sleep(200 * time.Millisecond)

	idx, cnt, ok := read()
	if !ok {
		fmt.Fprintln(os.Stderr, "cursor not readable — is the select screen up?")
		os.Exit(1)
	}
	target = ((target % cnt) + cnt) % cnt
	fmt.Printf("start: %s cursor=%d target=%d count=%d\n", list, idx, target, cnt)

	for step := 0; step < cnt+6; step++ {
		idx, cnt, _ = read()
		right, remaining := carouselNav(idx, target, cnt)
		if remaining == 0 {
			break
		}
		key := "Right"
		if !right {
			key = "Left"
		}
		fmt.Printf("step %d: cursor=%d press %s (%d left)\n", step, idx, key, remaining)
		_ = inj.Tap(key)
		// wait for the confirmed settled move
		prev := idx
		for w := 0; w < 30; w++ {
			time.Sleep(70 * time.Millisecond)
			if s, _, ok := read(); ok && s != prev {
				break
			}
		}
	}
	final, _, _ := read()
	if final != target {
		fmt.Printf("FAIL: landed on %d, target %d\n", final, target)
		os.Exit(1)
	}
	fmt.Printf("LANDED: %s cursor=%d == target=%d\n", list, final, target)
}

// carouselNav mirrors hostrunner.carouselNav (shorter way around the wrapping ring).
func carouselNav(cursor, target, count int) (right bool, remaining int) {
	if count <= 0 {
		return true, 0
	}
	fwd := ((target-cursor)%count + count) % count
	if fwd == 0 {
		return true, 0
	}
	if bwd := count - fwd; bwd < fwd {
		return false, bwd
	}
	return true, fwd
}

// enumMode reads the create-game map/gametype ENUMERATION (built-in ustr lists,
// the player-picker source) in-container and prints the counts + the gametype
// custom-variant prefix (live widget count − enumerated count) — the shift the
// runner applies to a name-based gametype pick.
func enumMode(sock string) {
	inst := &xemu.Instance{Name: "enum", QMPSock: sock}
	if err := inst.Init(haloce.AllLowGVAs); err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	defer inst.Close()
	r := haloce.NewReader(inst, "enum", haloce.BaselineOffsets())
	opts := r.EnumerateLobby()
	cur := r.ReadLobbyCursor()
	fmt.Printf("enumerate available=%v: maps=%d gametypes=%d\n", opts.Available, len(opts.Maps), len(opts.Gametypes))
	if len(opts.Gametypes) > 0 {
		fmt.Printf("  first built-in gametype: [%d]=%q\n", opts.Gametypes[0].Steps, opts.Gametypes[0].Name)
	}
	fmt.Printf("live widget: map=%d gametype=%d\n", cur.MapCount, cur.GametypeCount)
	fmt.Printf("=> gametype custom prefix = %d - %d = %d\n", cur.GametypeCount, len(opts.Gametypes), cur.GametypeCount-len(opts.Gametypes))
}
