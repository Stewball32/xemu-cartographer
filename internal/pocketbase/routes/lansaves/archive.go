package lansaves

import (
	"archive/tar"
	"archive/zip"
	"bytes"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// archiveTar packs a SaveSet into a tar archive whose entries are laid out at
// the exact FATX path a LAN client should write them to:
//
//	UDATA/<titleID>/<dir>/blam.lst
//	UDATA/<titleID>/<dir>/SaveMeta.xbx
//
// so the client can unpack relative to the Xbox E:\ root.
func archiveTar(set *halosave.SaveSet) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range set.Files {
		hdr := &tar.Header{
			Name: set.FatxDir + "/" + f.Name,
			Mode: 0o644,
			Size: int64(len(f.Data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// archiveZip is the same layout as archiveTar but as a zip (friendlier for a
// human downloading from the browser editor).
func archiveZip(set *halosave.SaveSet) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range set.Files {
		w, err := zw.Create(set.FatxDir + "/" + f.Name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.Data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
