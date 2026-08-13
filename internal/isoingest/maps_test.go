package isoingest

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeMap builds a minimal Halo-1 .map header (head/foot magic, name @0x20,
// type @0x60) — enough for the header parse.
func writeFakeMap(t *testing.T, dir, filename, internalName string, mapType uint32) {
	t.Helper()
	buf := make([]byte, mapHeaderSize)
	binary.LittleEndian.PutUint32(buf[0:], mapHeadMagic)
	binary.LittleEndian.PutUint32(buf[0x7FC:], mapFootMagic)
	copy(buf[mapOffName:], internalName)
	binary.LittleEndian.PutUint32(buf[mapOffType:], mapType)
	if err := os.WriteFile(filepath.Join(dir, filename), buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

func TestParseMapHeader(t *testing.T) {
	dir := t.TempDir()
	writeFakeMap(t, dir, "bloodgulch.map", "bloodgulch", 1)
	mi, err := parseMapHeader(filepath.Join(dir, "bloodgulch.map"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if mi.Name != "bloodgulch" || mi.Type != "multiplayer" || mi.Filename != "bloodgulch.map" {
		t.Errorf("got %+v", mi)
	}

	// Bad magic → error.
	bad := filepath.Join(dir, "bad.map")
	os.WriteFile(bad, make([]byte, mapHeaderSize), 0o644)
	if _, err := parseMapHeader(bad); err == nil {
		t.Error("zeroed header should be rejected")
	}
	// Too short → error.
	short := filepath.Join(dir, "short.map")
	os.WriteFile(short, []byte("head"), 0o644)
	if _, err := parseMapHeader(short); err == nil {
		t.Error("short file should be rejected")
	}
}

func TestParseMapList(t *testing.T) {
	tree := t.TempDir()
	mapsDir := filepath.Join(tree, "maps")
	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeMap(t, mapsDir, "ui.map", "ui", 2)
	writeFakeMap(t, mapsDir, "atlas.map", "atlas", 1)
	writeFakeMap(t, mapsDir, "a10.map", "a10", 0)
	// A non-.map and an unparseable .map are both skipped, not fatal.
	os.WriteFile(filepath.Join(mapsDir, "readme.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(mapsDir, "corrupt.map"), make([]byte, 16), 0o644)

	list := ParseMapList(tree)
	if len(list) != 3 {
		t.Fatalf("expected 3 parseable maps, got %d: %+v", len(list), list)
	}
	// Sorted by filename.
	if list[0].Filename != "a10.map" || list[0].Type != "campaign" {
		t.Errorf("first should be a10/campaign, got %+v", list[0])
	}
	byName := map[string]MapInfo{}
	for _, m := range list {
		byName[m.Name] = m
	}
	if byName["atlas"].Type != "multiplayer" || byName["ui"].Type != "ui" {
		t.Errorf("type map wrong: %+v", byName)
	}

	// Missing maps/ dir → empty, no panic.
	if got := ParseMapList(t.TempDir()); len(got) != 0 {
		t.Errorf("no maps/ should yield empty, got %d", len(got))
	}
}
