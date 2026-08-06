package isoingest

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildFakeXBE writes a minimal, structurally valid XBE: magic, base address,
// certificate VA, and the title id at cert+0x08.
func buildFakeXBE(t *testing.T, dir, name string, base, certVA, titleID uint32) string {
	t.Helper()
	size := int(certVA-base) + certOffTitleID + 4
	if size < xbeOffCertAddr+4 {
		size = xbeOffCertAddr + 4
	}
	b := make([]byte, size)
	copy(b, xbeMagic)
	binary.LittleEndian.PutUint32(b[xbeOffBaseAddr:], base)
	binary.LittleEndian.PutUint32(b[xbeOffCertAddr:], certVA)
	binary.LittleEndian.PutUint32(b[int(certVA-base)+certOffTitleID:], titleID)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("write fake xbe: %v", err)
	}
	return p
}

// TestTitleIDFromXBE parses the canonical Halo CE title id (0x4D530004) from a
// synthetic XBE laid out like the real one (base 0x10000, cert just past the
// fixed header).
func TestTitleIDFromXBE(t *testing.T) {
	dir := t.TempDir()
	p := buildFakeXBE(t, dir, "default.xbe", 0x10000, 0x10000+0x200, 0x4D530004)
	got, err := TitleIDFromXBE(p)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != "4d530004" {
		t.Errorf("title id = %q, want 4d530004 (lowercase hex, catalog format)", got)
	}
}

// TestTitleIDFromXBE_Rejects covers the failure modes: wrong magic, truncated
// file, certificate below base.
func TestTitleIDFromXBE_Rejects(t *testing.T) {
	dir := t.TempDir()

	bad := filepath.Join(dir, "not-an-xbe.bin")
	if err := os.WriteFile(bad, make([]byte, 0x200), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := TitleIDFromXBE(bad); err == nil {
		t.Error("zeroed magic should be rejected")
	}

	short := filepath.Join(dir, "short.xbe")
	if err := os.WriteFile(short, []byte(xbeMagic), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := TitleIDFromXBE(short); err == nil {
		t.Error("truncated header should be rejected")
	}

	// certVA < base
	b := make([]byte, xbeOffCertAddr+4)
	copy(b, xbeMagic)
	binary.LittleEndian.PutUint32(b[xbeOffBaseAddr:], 0x10000)
	binary.LittleEndian.PutUint32(b[xbeOffCertAddr:], 0x100)
	inv := filepath.Join(dir, "inv.xbe")
	if err := os.WriteFile(inv, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := TitleIDFromXBE(inv); err == nil {
		t.Error("cert VA below base should be rejected")
	}
}

// TestTitleIDFromTree finds default.xbe case-insensitively at the tree root and
// errors when absent.
func TestTitleIDFromTree(t *testing.T) {
	dir := t.TempDir()
	buildFakeXBE(t, dir, "Default.XBE", 0x10000, 0x10000+0x200, 0x4D530064)
	got, err := TitleIDFromTree(dir)
	if err != nil {
		t.Fatalf("tree parse: %v", err)
	}
	if got != "4d530064" {
		t.Errorf("title id = %q, want 4d530064", got)
	}
	if _, err := TitleIDFromTree(t.TempDir()); err == nil {
		t.Error("empty tree should error")
	}
}
