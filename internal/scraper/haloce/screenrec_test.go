package haloce

import (
	"encoding/binary"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// Screen-record resolve (screenrec.go) against a synthetic memory image: the
// current/back record pointers are low globals, the record's tag id round-trips
// through a fake cache tag table in the high window, and every failure mode
// (stale id, wrong group, garbage pointer) degrades to "" — never a wrong path.
//
// Layout (xemu.NewTestInstance: base 0, so a high GVA g reads file offset
// g-0x80000000; low GVAs resolve through the supplied map):
//
//	high 0x80000100  tag header {tag_array=0x80000200, count=3}
//	high 0x80000200  tag entries (0x20 stride):
//	   [0] DeLa handle 0xE1750000 name→0x80000400 (mp submenu screen)
//	   [1] DeLa handle 0xE2340001 name→0x80000500 (root main menu)
//	   [2] ustr handle 0xE0000002               (wrong group)
//	low  0x2DF1C4 → file 0x2100  tag header ptr
//	low  0x2E4000 → file 0x2000  cur-rec ptr
//	low  0x2E4010 → file 0x2010  back-rec ptr
//	low  0x4B2000 → file 0x3000  the record ARENA page base (registering the
//	     page is what lets a record slot at 0x4B2C48 resolve through
//	     lowHVADynamic's page fallback with no QMP)
const (
	trHeaderGVA   = 0x80000100
	trArrayGVA    = 0x80000200
	trNameSubGVA  = 0x80000400
	trNameRootGVA = 0x80000500

	trSubPath  = `ui\shell\main_menu\multiplayer_type_select\multiplayer_type_select_screen`
	trRootPath = `ui\shell\main_menu\main_menu`

	trHandleSub  = 0xE1750000 // tag idx 0
	trHandleRoot = 0xE2340001 // tag idx 1
	trHandleUstr = 0xE0000002 // tag idx 2 — not a DeLa entry

	trCurRecSlot = 0x4B2C48 // record slot inside the widget-instance arena
)

// screenRecRAM builds the base image: tag table + name strings + the tag-header
// low pointer. Callers then poke the cur/back globals and record slots.
func screenRecRAM(t *testing.T) []byte {
	t.Helper()
	ram := make([]byte, 0x4000)
	put32 := func(off uint32, v uint32) { binary.LittleEndian.PutUint32(ram[off:], v) }

	put32(trHeaderGVA-0x80000000+OffTagHeaderTagArray, trArrayGVA)
	put32(trHeaderGVA-0x80000000+OffTagHeaderTagCount, 3)
	entry := func(idx uint32, group string, handle, namePtr uint32) {
		base := trArrayGVA - 0x80000000 + idx*ConstTagEntrySize
		// Group fourCC is stored byte-reversed.
		ram[base+0], ram[base+1], ram[base+2], ram[base+3] = group[3], group[2], group[1], group[0]
		put32(base+OffTagHandle, handle)
		put32(base+OffTagNamePtr, namePtr)
	}
	entry(0, tagGroupDela, trHandleSub, trNameSubGVA)
	entry(1, tagGroupDela, trHandleRoot, trNameRootGVA)
	entry(2, "ustr", trHandleUstr, trNameSubGVA)
	copy(ram[trNameSubGVA-0x80000000:], trSubPath+"\x00")
	copy(ram[trNameRootGVA-0x80000000:], trRootPath+"\x00")

	put32(0x2100, trHeaderGVA) // *AddrTagHeaderPtr → tag header
	return ram
}

// setScreenRec pokes the cur/back globals (and, when curRec lands on the mapped
// page, the record slot's tag id) into the image.
func setScreenRec(ram []byte, curRec, backRec, tagID uint32) {
	binary.LittleEndian.PutUint32(ram[0x2000:], curRec)
	binary.LittleEndian.PutUint32(ram[0x2010:], backRec)
	if curRec&^uint32(0xFFF) == 0x4B2000 && curRec&0xFFF <= 0xFE4 {
		binary.LittleEndian.PutUint32(ram[0x3000+(curRec&0xFFF):], tagID)
	}
}

// screenRecReader builds a Reader over the image with the baseline offset set.
// The record globals are VERSIONED, so the set is bound first and its addresses
// key the translation table — the reader reads through r.off, and a set that
// moved these would move the test with it.
func screenRecReader(t *testing.T, ram []byte) *Reader {
	t.Helper()
	off, err := OffsetsFromSet(offsets.Baseline("haloce"))
	if err != nil {
		t.Fatalf("bind baseline offsets: %v", err)
	}
	inst, cleanup, err := xemu.NewTestInstance("screenrec", ram, map[uint32]int64{
		off.AddrTagHeaderPtr:       0x2100,
		off.AddrUiCurrentScreenRec: 0x2000,
		off.AddrUiBackScreenRec:    0x2010,
		0x4B2000:                   0x3000, // arena page base (the record slot's page)
	})
	if err != nil {
		t.Fatalf("NewTestInstance: %v", err)
	}
	t.Cleanup(cleanup)
	return NewReader(inst, "screenrec", off)
}

func TestReadUiScreenResolvesRecordTag(t *testing.T) {
	ram := screenRecRAM(t)
	setScreenRec(ram, trCurRecSlot, 0x2E4050, trHandleSub)
	r := screenRecReader(t, ram)

	path, cur, back, _, _ := r.readUiScreen()
	if path != trSubPath {
		t.Fatalf("path = %q, want %q", path, trSubPath)
	}
	if cur != trCurRecSlot || back != 0x2E4050 {
		t.Fatalf("cur/back = 0x%X/0x%X, want 0x%X/0x2E4050", cur, back, trCurRecSlot)
	}
	if _, ok := r.screenTagPaths[trHandleSub]; !ok {
		t.Fatal("resolve must cache by TAG ID")
	}
	if p := r.delaPathForTagID(trHandleSub); p != trSubPath {
		t.Fatalf("cached resolve = %q, want %q", p, trSubPath)
	}
}

// SLOT REUSE (the doc's headline rule): the SAME record address holds a different
// screen's tag id on a later visit — the resolve must follow the TAG, so a cache
// keyed by anything address-shaped would return the previous screen here.
func TestReadUiScreenFollowsReusedSlot(t *testing.T) {
	ram := screenRecRAM(t)
	setScreenRec(ram, trCurRecSlot, 0, trHandleRoot)
	r := screenRecReader(t, ram)
	// Warm the tag cache with the OTHER screen first (as a prior visit would).
	if p := r.delaPathForTagID(trHandleSub); p != trSubPath {
		t.Fatalf("warm resolve = %q, want %q", p, trSubPath)
	}
	path, _, back, _, _ := r.readUiScreen()
	if path != trRootPath {
		t.Fatalf("reused slot: path = %q, want %q (must re-resolve the tag)", path, trRootPath)
	}
	if back != 0 {
		t.Fatalf("back = 0x%X, want 0 (root)", back)
	}
}

func TestReadUiScreenRejectsBadRecords(t *testing.T) {
	cases := []struct {
		name   string
		curRec uint32 // value of the cur-rec global
		tagID  uint32 // content of the record slot (when on the mapped page)
	}{
		{"no current screen (cold pool)", 0, 0},
		{"pointer below the arena", 0x2E40C8, 0},
		{"pointer above the arena", 0x4C2B80, 0},
		{"page-straddling record", 0x4B2FFC, 0},
		{"stale tag id (entry doesn't round-trip)", trCurRecSlot, 0xDEAD0001},
		{"non-DeLa entry (wrong group)", trCurRecSlot, trHandleUstr},
		{"tag idx out of range", trCurRecSlot, 0xE0000030},
		{"unregistered page (dynamic translate fails w/o QMP)", 0x4B80C8, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ram := screenRecRAM(t)
			setScreenRec(ram, c.curRec, 0, c.tagID)
			r := screenRecReader(t, ram)
			if path, _, _, _, _ := r.readUiScreen(); path != "" {
				t.Fatalf("path = %q, want \"\" (no signal beats a wrong screen)", path)
			}
		})
	}
}
