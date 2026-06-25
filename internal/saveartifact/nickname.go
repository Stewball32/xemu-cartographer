package saveartifact

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/Stewball32/xemu-cartographer/internal/consolename"
	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// ConsoleNameBundle builds the deployable Xbox console name E:\UDATA\NICKNAME.XBN
// (plaintext, no checksum) from a gamertag, via the shared internal/consolename
// builder (the same one the podman provisioner writes into a container overlay).
//
// This is the box's dashboard / system-link identity — NOT the Halo: CE player
// profile (CE has its own profiles; their format is being re-investigated). The
// bundle tars the file at UDATA/NICKNAME.XBN so the LAN client unpacks it
// relative to the Xbox E:\ root. No signing — NICKNAME.XBN carries no checksum.
func ConsoleNameBundle(gamertag string) (*Bundle, error) {
	name := consolename.Sanitize(gamertag)
	if name == "" {
		return nil, fmt.Errorf("saveartifact: gamertag %q has no printable-ASCII characters for the console name", gamertag)
	}
	payload := consolename.BuildXBN(name)
	set := &halosave.SaveSet{
		Title:   "xbox",
		Kind:    "console-name",
		FatxDir: "UDATA",
		Files:   []halosave.SaveFile{saveFile("NICKNAME.XBN", payload)},
		Digest: halosave.DigestStatus{
			Mode:     halosave.DigestMode("unsigned"),
			Resolved: true, // nothing to sign — plaintext file
			Note:     "NICKNAME.XBN is plaintext with no checksum or signature.",
		},
		Warnings: []string{
			"The Xbox console name (E:\\UDATA\\NICKNAME.XBN) — the box's dashboard / " +
				"system-link identity. It is NOT the Halo: CE player profile.",
		},
	}
	tarBytes, err := tarSet(set)
	if err != nil {
		return nil, err
	}
	return &Bundle{Set: set, Tar: tarBytes}, nil
}

// saveFile constructs a halosave.SaveFile with its integrity metadata for files
// built outside halosave.Build (e.g. the CE console name).
func saveFile(name string, data []byte) halosave.SaveFile {
	sum := sha1.Sum(data)
	return halosave.SaveFile{Name: name, Size: len(data), SHA1: hex.EncodeToString(sum[:]), Data: data}
}
