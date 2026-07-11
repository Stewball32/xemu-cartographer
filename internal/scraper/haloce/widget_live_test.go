package haloce

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// widget_live_test.go is a LIVE integration harness for the CE menu-widget cursor
// read + closed-loop carousel nav. It is env-gated and skipped in CI (no xemu):
//
//	CE_LIVE_SOCK=/run/user/1000/xemu-qmp-ce-nav.sock \
//	  go test ./internal/scraper/haloce/ -run TestLiveWidget -v
//
// TestLiveWidgetRead just reads + logs both cursors (cross-check vs ce_widget.py).
// TestLiveCarouselNav additionally drives the carousel to a target via a pad FIFO:
//
//	CE_LIVE_SOCK=... CE_NAV_FIFO=/run/user/1000/xemu-harness/ce-nav/p1.fifo \
//	  CE_NAV_LIST=map CE_NAV_TARGET=3 \
//	  go test ./internal/scraper/haloce/ -run TestLiveCarouselNav -v

func liveReader(t *testing.T) *Reader {
	t.Helper()
	sock := os.Getenv("CE_LIVE_SOCK")
	if sock == "" {
		t.Skip("set CE_LIVE_SOCK to run the live widget harness")
	}
	inst := &xemu.Instance{Name: "ce-live", QMPSock: sock}
	if err := inst.Init(AllLowGVAs); err != nil {
		t.Fatalf("instance init: %v", err)
	}
	t.Cleanup(inst.Close)
	return NewReader(inst, "ce-live")
}

// rawSel reads the raw (sel,count,found) for a named list widget, bypassing the
// count>0 gate so we can compare against ce_widget.py even off the active screen.
func (r *Reader) rawSel(t *testing.T, tagPath string) (sel, count int, found bool) {
	t.Helper()
	h := r.resolveDelaHandles(tagPath)[tagPath]
	if h == 0 {
		return 0, 0, false
	}
	heap, err := r.inst.Mem.ReadBytes(ConstUiWidgetHeapGVALo, int(ConstUiWidgetHeapGVAHi-ConstUiWidgetHeapGVALo))
	if err != nil {
		t.Fatalf("heap read: %v", err)
	}
	return findWidgetSelection(heap, h)
}

// TestLiveWidgetDump enumerates EVERY heap match for the two select-list handles
// (any byte alignment) so we can see the freed vs live blocks the way ce_widget.py
// does. Diagnostic only.
func TestLiveWidgetDump(t *testing.T) {
	r := liveReader(t)
	heap, err := r.inst.Mem.ReadBytes(ConstUiWidgetHeapGVALo, int(ConstUiWidgetHeapGVAHi-ConstUiWidgetHeapGVALo))
	if err != nil {
		t.Fatal(err)
	}
	for _, tp := range []string{TagPathMPMapSelectList, TagPathGametypeSelectList} {
		h := r.resolveDelaHandles(tp)[tp]
		t.Logf("%s handle=0x%08X", tp, h)
		if h == 0 {
			continue
		}
		hb := []byte{byte(h), byte(h >> 8), byte(h >> 16), byte(h >> 24)}
		for o := 0; o+4 <= len(heap); o++ { // BYTE-granular, like ce_widget.py
			if heap[o] != hb[0] || heap[o+1] != hb[1] || heap[o+2] != hb[2] || heap[o+3] != hb[3] {
				continue
			}
			nb := o - int(OffUiWidgetDefTagHandle)
			if nb < 0 {
				continue
			}
			hdr := leU32Int(heap, nb)
			valid := hdr&ConstUiWidgetHeaderMask == ConstUiWidgetHeaderFlag
			sel := int(int32(leU32Int(heap, nb+int(OffUiWidgetSelectedIndex))))
			cnt := int(int32(leU32Int(heap, nb+int(OffUiWidgetItemCount))))
			t.Logf("  match@off=0x%X (align%%4=%d) nb-gva=0x%08X hdr=0x%08X valid=%v sel=%d cnt=%d",
				o, o%4, ConstUiWidgetHeapGVALo+uint32(nb), hdr, valid, sel, cnt)
		}
	}
}

// TestLiveEnumerate prints the ustr-enumerated map/gametype lists (the player
// picker's source) with their absolute Steps, alongside the live widget counts —
// to expose the gametype custom-variant prefix (widget count vs enumerated count).
func TestLiveEnumerate(t *testing.T) {
	r := liveReader(t)
	opts := r.EnumerateLobby()
	t.Logf("enumerate available=%v: %d maps, %d gametypes", opts.Available, len(opts.Maps), len(opts.Gametypes))
	for _, m := range opts.Maps {
		t.Logf("  map[%d] = %q", m.Steps, m.Name)
	}
	for _, g := range opts.Gametypes {
		t.Logf("  gt[%d] = %q", g.Steps, g.Name)
	}
	cur := r.ReadLobbyCursor()
	t.Logf("live widget counts: map=%d gametype=%d", cur.MapCount, cur.GametypeCount)
	t.Logf("=> gametype custom-prefix = widgetCount(%d) - enumCount(%d) = %d",
		cur.GametypeCount, len(opts.Gametypes), cur.GametypeCount-len(opts.Gametypes))
}

func TestLiveWidgetRead(t *testing.T) {
	r := liveReader(t)
	ms, mc, mf := r.rawSel(t, TagPathMPMapSelectList)
	gs, gc, gf := r.rawSel(t, TagPathGametypeSelectList)
	t.Logf("Go widget read: map  handle-found=%v sel=%d count=%d", mf, ms, mc)
	t.Logf("Go widget read: gtyp handle-found=%v sel=%d count=%d", gf, gs, gc)
	cur := r.ReadLobbyCursor()
	t.Logf("ReadLobbyCursor: %+v", cur)
	if !mf && !gf {
		t.Log("neither select-list widget resolved — not on/past the create-game screens?")
	}
}

// TestLiveCarouselNav drives the chosen carousel to CE_NAV_TARGET using the exact
// closed-loop the runner uses (carouselNav + re-read to confirm), then asserts the
// live cursor == target. Proves map AND gametype navigation land from a
// non-deterministic start (incl. wrap) directly against live guest memory.
func TestLiveCarouselNav(t *testing.T) {
	r := liveReader(t)
	fifo := os.Getenv("CE_NAV_FIFO")
	if fifo == "" {
		t.Skip("set CE_NAV_FIFO (pad command pipe) to drive the carousel")
	}
	list := os.Getenv("CE_NAV_LIST") // "map" | "gametype"
	target, _ := strconv.Atoi(os.Getenv("CE_NAV_TARGET"))

	tagPath := TagPathMPMapSelectList
	if list == "gametype" || list == "gt" {
		tagPath = TagPathGametypeSelectList
	}

	// read waits for a SETTLED cursor: count>0 and sel in [0,count). The +0x4C
	// field holds out-of-range garbage mid-scroll-animation, so poll until it
	// settles (mirrors the ReadLobbyCursor liveIndex gate + the runner's re-read).
	read := func() (sel, count int) {
		for i := 0; i < 40; i++ {
			s, c, found := r.rawSel(t, tagPath)
			if !found {
				t.Fatalf("%s widget not found — navigate to its select screen first", list)
			}
			if c > 0 && s >= 0 && s < c {
				return s, c
			}
			time.Sleep(60 * time.Millisecond)
		}
		t.Fatalf("%s cursor never settled to a valid index", list)
		return 0, 0
	}

	press := func(tok string) {
		f, err := os.OpenFile(fifo, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("open fifo: %v", err)
		}
		defer f.Close()
		if _, err := f.WriteString(tok + "\n"); err != nil {
			t.Fatalf("write fifo: %v", err)
		}
	}

	sel, count := read()
	if count <= 0 {
		t.Fatalf("%s list not active (count=%d) — reach its select screen", list, count)
	}

	// FULL PRODUCTION PATH: when CE_NAV_NAME is set, resolve the target the way the
	// play API + runner do — enumeration IndexOf(name) → ustr index, then + the
	// gametype custom-variant prefix (widget count − enumerated count) → widget
	// index. Proves a pick BY NAME lands on the right card, not just a raw index.
	if name := os.Getenv("CE_NAV_NAME"); name != "" {
		opts := r.EnumerateLobby()
		options := opts.Maps
		enumLen := len(opts.Maps)
		if tagPath == TagPathGametypeSelectList {
			options = opts.Gametypes
			enumLen = len(opts.Gametypes)
		}
		idx := -1
		for _, o := range options {
			if o.Name == name {
				idx = o.Steps
			}
		}
		if idx < 0 {
			t.Fatalf("%q not in the enumerated %s list", name, list)
		}
		prefix := 0
		if tagPath == TagPathGametypeSelectList && count > enumLen {
			prefix = count - enumLen // custom variants prepended
		}
		target = idx + prefix
		t.Logf("by-name %q: enumIndex=%d + prefix=%d → widget target=%d", name, idx, prefix, target)
	}

	target = ((target % count) + count) % count
	t.Logf("start: %s cursor=%d target=%d count=%d", list, sel, target, count)

	// navMove mirrors hostrunner.carouselNav: shorter way around the wrapping ring.
	navMove := func(cursor, target, count int) (right bool, remaining int) {
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

	for step := 0; step < count+6; step++ {
		sel, count = read() // settled cursor
		right, remaining := navMove(sel, target, count)
		if remaining == 0 {
			break
		}
		key := "right"
		if !right {
			key = "left"
		}
		t.Logf("step %d: cursor=%d → press %s (%d left)", step, sel, key, remaining)
		press(key)
		// Wait for the confirmed move: re-read until the settled cursor changes.
		for w := 0; w < 30; w++ {
			time.Sleep(70 * time.Millisecond)
			if s, _ := read(); s != sel {
				break
			}
		}
	}

	final, _ := read()
	if final != target {
		t.Fatalf("nav did NOT land: final cursor=%d, target=%d", final, target)
	}
	t.Logf("LANDED: %s cursor=%d == target=%d ✓", list, final, target)
}
