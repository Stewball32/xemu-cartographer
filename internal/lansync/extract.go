package lansync

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/diskspace"
)

// ExtractISO extracts the XISO named `filename` (resolved under cfg.ISODir) into
// a per-record cache dir (<cfg.ExtractDir>/isos/<recordID>) via extract-xiso,
// then measures the tree's FATX footprint. Returns the tree dir + footprint.
//
// The cache dir is wiped + recreated first, so re-extraction is idempotent
// (eviction is just removing the dir). The binary the ISO row points at is
// immutable (isos_immutable), so the tree is a pure function of that binary — no
// content hashing needed.
func ExtractISO(cfg Config, filename, recordID string) (treeDir string, footprint uint64, err error) {
	if strings.ContainsAny(filename, `/\`) || filename == "" || strings.HasPrefix(filename, ".") {
		return "", 0, fmt.Errorf("invalid iso filename %q", filename)
	}
	isoPath := filepath.Join(cfg.ISODir, filename)
	if _, statErr := os.Stat(isoPath); statErr != nil {
		return "", 0, fmt.Errorf("iso not found at %s: %w", isoPath, statErr)
	}

	treeDir = filepath.Join(cfg.ExtractDir, "isos", recordID)
	if err = os.RemoveAll(treeDir); err != nil {
		return "", 0, fmt.Errorf("clear cache dir: %w", err)
	}
	if err = os.MkdirAll(treeDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("mkdir cache dir: %w", err)
	}

	// extract-xiso -x -d <treeDir> <isoPath> — expand the disc into treeDir.
	cmd := exec.Command(cfg.extractXISO(), "-x", "-d", treeDir, isoPath)
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		return "", 0, fmt.Errorf("extract-xiso: %w: %s", runErr, strings.TrimSpace(string(out)))
	}

	footprint, err = FootprintTree(treeDir, cfg.FATXCluster)
	if err != nil {
		return "", 0, err
	}
	return treeDir, footprint, nil
}

// extractXISO returns the configured extract-xiso command (default "extract-xiso").
func (c Config) extractXISO() string {
	if c.ExtractXISOCmd != "" {
		return c.ExtractXISOCmd
	}
	return "extract-xiso"
}

// FootprintTree walks dir and returns the FATX cluster-rounded on-disk size of
// all regular files (the drive-fill footprint the client charges against free
// space).
func FootprintTree(dir string, cluster int) (uint64, error) {
	var sizes []int
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode().IsRegular() {
			sizes = append(sizes, int(info.Size()))
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return diskspace.FATXFootprint(sizes, cluster), nil
}

// TarDir packs every regular file under dir into a tar, with paths RELATIVE to
// dir (so the client unpacks the tree at its chosen destination). Returns the
// tar bytes. Used by the game-download handler.
func TarDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		hdr := &tar.Header{Name: filepath.ToSlash(rel), Mode: 0o644, Size: info.Size()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ZipFootprint reads a zip from r (of length size) and returns the FATX
// cluster-rounded UNCOMPRESSED footprint of its entries — what the app occupies
// on the FATX drive once laid down. Used by the app-extraction hook (apps
// download as the stored zip, but the drive-fill math needs the unpacked size).
func ZipFootprint(r io.ReaderAt, size int64, cluster int) (uint64, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return 0, fmt.Errorf("open zip: %w", err)
	}
	var sizes []int
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		sizes = append(sizes, int(f.UncompressedSize64))
	}
	return diskspace.FATXFootprint(sizes, cluster), nil
}
