package isoingest

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Title-ID auto-extraction: every ingest already produces the extracted disc
// tree, whose default.xbe carries the Xbox title certificate. Parsing it here
// means organizers never hand-enter a title id (and can't typo one) — the
// catalog stores what the disc actually says.
//
// XBE layout (original Xbox executable):
//
//	0x000  magic "XBEH"
//	0x104  base address (u32 LE)      — the VA the header is mapped at
//	0x118  certificate address (u32)  — VA of the certificate
//	cert+0x08  title id (u32 LE)
//
// The certificate's file offset is certVA - baseVA (the header region is mapped
// 1:1 from the file start). Same structure the scraper reads live at GVA
// 0x00010000 (scraper.ReadTitleID) — this is the at-rest twin of that read.
const (
	xbeMagic       = "XBEH"
	xbeOffBaseAddr = 0x104
	xbeOffCertAddr = 0x118
	certOffTitleID = 0x08
	xbeHeaderRead  = 0x1000 // plenty for header + certificate
)

// TitleIDFromXBE reads the title id from an XBE file, returned as the catalog's
// canonical lowercase 8-hex-digit string (e.g. "4d530004").
func TitleIDFromXBE(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, xbeHeaderRead)
	n, err := f.ReadAt(buf, 0)
	if n < xbeOffCertAddr+4 {
		return "", fmt.Errorf("xbe too short (%d bytes): %v", n, err)
	}
	buf = buf[:n]
	if string(buf[:4]) != xbeMagic {
		return "", fmt.Errorf("not an XBE (magic %q)", buf[:4])
	}
	base := binary.LittleEndian.Uint32(buf[xbeOffBaseAddr:])
	certVA := binary.LittleEndian.Uint32(buf[xbeOffCertAddr:])
	if certVA < base {
		return "", fmt.Errorf("certificate VA 0x%X below base 0x%X", certVA, base)
	}
	certOff := int64(certVA - base)
	title := make([]byte, 4)
	if _, err := f.ReadAt(title, certOff+certOffTitleID); err != nil {
		return "", fmt.Errorf("read title id at 0x%X: %w", certOff+certOffTitleID, err)
	}
	return fmt.Sprintf("%08x", binary.LittleEndian.Uint32(title)), nil
}

// TitleIDFromTree finds the boot XBE in an extracted disc tree and parses its
// title id. Looks for default.xbe (any case) at the tree root — the standard
// boot executable extract-xiso lays down.
func TitleIDFromTree(treeDir string) (string, error) {
	entries, err := os.ReadDir(treeDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), "default.xbe") {
			return TitleIDFromXBE(filepath.Join(treeDir, e.Name()))
		}
	}
	return "", fmt.Errorf("no default.xbe at tree root %s", treeDir)
}
