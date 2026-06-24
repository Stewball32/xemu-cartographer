package halosave

import (
	"errors"
	"strings"
	"unicode/utf16"
)

// SaveMeta is the display-name sidecar written next to every save payload
// (E:\UDATA\<title>\<dir>\SaveMeta.xbx). It is plain UTF-16LE text with a BOM
// and carries NO checksum or signature, so it is trivially and fully
// generatable:
//
//	FF FE                      <- UTF-16LE BOM
//	"Name=<display>\r\n"       <- UTF-16LE
var saveMetaBOM = []byte{0xFF, 0xFE}

// SaveMetaParse extracts the display name from a SaveMeta.xbx byte slice.
func SaveMetaParse(b []byte) (string, error) {
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		return "", errors.New("halosave: SaveMeta.xbx missing UTF-16LE BOM")
	}
	if len(b)%2 != 0 {
		return "", errors.New("halosave: SaveMeta.xbx body is not 2-byte aligned")
	}
	units := make([]uint16, 0, (len(b)-2)/2)
	for i := 2; i+1 < len(b); i += 2 {
		units = append(units, uint16(b[i])|uint16(b[i+1])<<8)
	}
	text := string(utf16.Decode(units))
	if !strings.HasPrefix(text, "Name=") {
		return "", errors.New("halosave: unexpected SaveMeta body " + text)
	}
	return strings.TrimRight(text[len("Name="):], "\r\n"), nil
}

// SaveMetaBuild produces a SaveMeta.xbx for the given display name, byte-identical
// to the form Halo writes (BOM + "Name=<display>\r\n" in UTF-16LE).
func SaveMetaBuild(displayName string) []byte {
	body := utf16.Encode([]rune("Name=" + displayName + "\r\n"))
	out := make([]byte, 2+len(body)*2)
	copy(out, saveMetaBOM)
	for i, u := range body {
		out[2+i*2] = byte(u)
		out[2+i*2+1] = byte(u >> 8)
	}
	return out
}
