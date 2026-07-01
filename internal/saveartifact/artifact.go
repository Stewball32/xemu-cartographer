// Package saveartifact turns a halosave BuildRequest into a single deployable
// artifact: the generated FATX save dir packed as a tar (laid out at the exact
// E:\ path the nxdk LAN client writes to), plus the SaveSet metadata.
//
// It is the shared seam between the generate-on-save record hooks
// (internal/pocketbase/hooks) — which store the tar on a PocketBase file field
// when a profile/gametype record is saved — and any other consumer that needs a
// ready-to-write bundle. Pure (no PB / no IO beyond in-memory tar), so it is
// unit-tested directly against halosave's real-sample templates.
package saveartifact

import (
	"archive/tar"
	"bytes"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
)

// Bundle is a generated save ready to store + serve.
type Bundle struct {
	// Set is the full halosave result (title/kind/fatx dir/files/digest/...).
	// Its SaveFile.Data bytes are json-omitted, so Set marshals to clean
	// metadata for the record's save_info field.
	Set *halosave.SaveSet
	// Tar is the save dir packed at UDATA/<titleID>/<dir>/<file> for each file,
	// so the client unpacks it relative to the Xbox E:\ root. This is the byte
	// stream stored on the record's save_bundle file field.
	Tar []byte
}

// FileMeta is one file's integrity metadata in a bundle's Info (no bytes).
type FileMeta struct {
	Name string `json:"name"`
	Size int    `json:"size"`
	SHA1 string `json:"sha1"`
}

// Info is the lean, display-friendly projection of a Bundle stored on a
// record's save_info JSON field. It deliberately drops the SaveSet's re-parsed
// payload (which carries raw bytes) so the column stays small.
type Info struct {
	Title    string                `json:"title"`
	Kind     string                `json:"kind"`
	TitleID  string                `json:"title_id"`
	DirName  string                `json:"dir_name"`
	FatxDir  string                `json:"fatx_dir"`
	Files    []FileMeta            `json:"files"`
	Digest   halosave.DigestStatus `json:"digest"`
	Total    int                   `json:"total_bytes"`
	Warnings []string              `json:"warnings,omitempty"`
}

// Info projects the bundle's SaveSet into the lean Info stored on a record.
func (b *Bundle) Info() Info {
	files := make([]FileMeta, len(b.Set.Files))
	for i, f := range b.Set.Files {
		files[i] = FileMeta{Name: f.Name, Size: f.Size, SHA1: f.SHA1}
	}
	return Info{
		Title:    b.Set.Title,
		Kind:     b.Set.Kind,
		TitleID:  b.Set.TitleID,
		DirName:  b.Set.DirName,
		FatxDir:  b.Set.FatxDir,
		Files:    files,
		Digest:   b.Set.Digest,
		Total:    b.Set.TotalBytes(),
		Warnings: b.Set.Warnings,
	}
}

// Build generates the save for req and packs it into a tar bundle. It returns
// halosave's error verbatim when the request is invalid (unknown title/kind,
// out-of-range appearance byte, etc.) so callers can reject the save.
func Build(req halosave.BuildRequest) (*Bundle, error) {
	set, err := halosave.Build(req)
	if err != nil {
		return nil, err
	}
	tarBytes, err := tarSet(set)
	if err != nil {
		return nil, err
	}
	return &Bundle{Set: set, Tar: tarBytes}, nil
}

// tarSet mirrors lansaves.archiveTar: one entry per file at its full FATX path.
func tarSet(set *halosave.SaveSet) ([]byte, error) {
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
