package saveartifact

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/consolename"
)

func TestCEProfileBundle(t *testing.T) {
	b, err := CEProfileBundle("CARTOG")
	if err != nil {
		t.Fatalf("CEProfileBundle: %v", err)
	}
	entries := tarEntries(t, b.Tar)
	data, ok := entries["UDATA/NICKNAME.XBN"]
	if !ok {
		t.Fatalf("tar missing UDATA/NICKNAME.XBN; have %v", keys(entries))
	}
	if len(data) != consolename.FileSize {
		t.Errorf("NICKNAME.XBN = %d bytes, want %d", len(data), consolename.FileSize)
	}
	if data[0] != 0x04 || data[2] != 'S' || data[3] != 'M' {
		t.Errorf("bad NICKNAME.XBN header: % x", data[:4])
	}
	if b.Set.FatxDir != "UDATA" {
		t.Errorf("fatx dir = %q, want UDATA", b.Set.FatxDir)
	}
	// No signing concern for a plaintext file.
	if !b.Set.Digest.Resolved {
		t.Error("NICKNAME.XBN bundle should report resolved (nothing to sign)")
	}
}

func TestCEProfileBundleRejectsEmpty(t *testing.T) {
	if _, err := CEProfileBundle("😀"); err == nil {
		t.Fatal("expected error for a gamertag with no printable-ASCII characters")
	}
}
