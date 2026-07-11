package haloce

import (
	"encoding/binary"
	"testing"
)

// putWidgetBlock writes a widget heap block at byte offset off: header
// (flag|size), the DeLa tag handle at +0x10, selected index at +0x4C, item count
// at +0x54. Mirrors the live layout so findWidgetSelection can be tested with no
// xemu.
func putWidgetBlock(heap []byte, off int, handle uint32, sel, count int32) {
	binary.LittleEndian.PutUint32(heap[off+int(OffUiWidgetBlockHeader):], ConstUiWidgetHeaderFlag|ConstUiWidgetBlockSize)
	binary.LittleEndian.PutUint32(heap[off+int(OffUiWidgetDefTagHandle):], handle)
	binary.LittleEndian.PutUint32(heap[off+int(OffUiWidgetSelectedIndex):], uint32(sel))
	binary.LittleEndian.PutUint32(heap[off+int(OffUiWidgetItemCount):], uint32(count))
}

func TestFindWidgetSelection(t *testing.T) {
	const handle = 0xE1230045

	t.Run("active block", func(t *testing.T) {
		heap := make([]byte, 0x1000)
		putWidgetBlock(heap, 0x200, handle, 7, 13)
		sel, count, ok := findWidgetSelection(heap, handle)
		if !ok || sel != 7 || count != 13 {
			t.Fatalf("got sel=%d count=%d ok=%v, want 7/13/true", sel, count, ok)
		}
	})

	t.Run("not found", func(t *testing.T) {
		heap := make([]byte, 0x1000)
		putWidgetBlock(heap, 0x200, 0xAABBCCDD, 3, 13)
		if _, _, ok := findWidgetSelection(heap, handle); ok {
			t.Fatal("should not match a different handle")
		}
	})

	t.Run("prefers active over stale", func(t *testing.T) {
		heap := make([]byte, 0x2000)
		// A freed block earlier in the heap keeps the handle but count 0 (stale idx).
		putWidgetBlock(heap, 0x100, handle, 99, 0)
		// The live instance later, with the real cursor + count.
		putWidgetBlock(heap, 0x800, handle, 5, 27)
		sel, count, ok := findWidgetSelection(heap, handle)
		if !ok || sel != 5 || count != 27 {
			t.Fatalf("got sel=%d count=%d ok=%v, want 5/27/true (active wins)", sel, count, ok)
		}
	})

	t.Run("handle match without valid header is ignored", func(t *testing.T) {
		heap := make([]byte, 0x1000)
		// Handle bytes present at a +0x10-aligned spot but the block header is junk.
		off := 0x300
		binary.LittleEndian.PutUint32(heap[off+int(OffUiWidgetDefTagHandle):], handle)
		binary.LittleEndian.PutUint32(heap[off:], 0x12345678) // not flag|size
		if _, _, ok := findWidgetSelection(heap, handle); ok {
			t.Fatal("a handle match without a valid block header must be rejected")
		}
	})

	t.Run("zero count reports not-active but returns the value", func(t *testing.T) {
		heap := make([]byte, 0x1000)
		putWidgetBlock(heap, 0x200, handle, 4, 0)
		sel, count, ok := findWidgetSelection(heap, handle)
		// ok is true (block found), but count==0 signals "not live" to ReadLobbyCursor.
		if !ok || sel != 4 || count != 0 {
			t.Fatalf("got sel=%d count=%d ok=%v, want 4/0/true", sel, count, ok)
		}
	})
}
