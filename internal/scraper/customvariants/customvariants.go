// Package customvariants enumerates a box's SAVED custom gametype variant names
// by reading them host-side off the container's overlay qcow2.
//
// This is the one gametype-list piece the guest can't surface from memory: the
// SELECT GAMETYPE carousel keeps only the rendered cards resident and streams
// the rest from disk, so a live memory read sees only a few. The complete set
// lives on the guest HDD (E:\UDATA\<title>\<dir>\blam.lst). We read it the same
// proven way the testrig does — reflink-snapshot the live overlay, FUSE-export
// it, and pyfatx-read each variant's signature-verified in-file name — via a
// small embedded Python helper (no Go FATX/qcow2 reader exists).
//
// The whole path is best-effort: if python3 / pyfatx / qemu-storage-daemon
// aren't present, or the overlay can't be read, Names returns an error and the
// caller falls back to the built-in gametypes only (no custom variants shown).
package customvariants

import (
	_ "embed"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

//go:embed fatx_gametypes.py
var helperScript []byte

// Names returns the custom gametype variant DISPLAY names saved on the box whose
// overlay qcow2 is overlayPath, in live-carousel order (FATX directory order —
// RUNTIME-VALIDATED to match the SELECT GAMETYPE carousel), each verified against
// the roamable-save signature so a corrupt/foreign file can't inject a garbage
// name. titleID is the 8-hex title (CE = "4d530004"). ctx bounds the whole read
// (snapshot + FUSE export + FATX walk); use a timeout — this touches the disk.
func Names(ctx context.Context, overlayPath, titleID string) ([]string, error) {
	if overlayPath == "" {
		return nil, errors.New("customvariants: empty overlay path")
	}
	if _, err := os.Stat(overlayPath); err != nil {
		return nil, fmt.Errorf("customvariants: overlay: %w", err)
	}
	f, err := os.CreateTemp("", "fatx_gametypes-*.py")
	if err != nil {
		return nil, fmt.Errorf("customvariants: temp helper: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(helperScript); err != nil {
		f.Close()
		return nil, fmt.Errorf("customvariants: write helper: %w", err)
	}
	f.Close()

	cmd := exec.CommandContext(ctx, "python3", f.Name(), overlayPath, titleID)
	out, err := cmd.Output()
	if err != nil {
		// The helper prints {"error":...} to stdout before a non-zero exit; surface
		// it when present so the log says WHY (no pyfatx, storage-daemon, …).
		if res, perr := parse(out); perr == nil && res.Error != "" {
			return nil, fmt.Errorf("customvariants: %s", res.Error)
		}
		return nil, fmt.Errorf("customvariants: run helper: %w", err)
	}
	res, err := parse(out)
	if err != nil {
		return nil, fmt.Errorf("customvariants: parse helper output: %w", err)
	}
	if res.Error != "" {
		return nil, fmt.Errorf("customvariants: %s", res.Error)
	}
	return res.Names, nil
}

type helperResult struct {
	Names []string `json:"names"`
	Error string   `json:"error"`
}

func parse(out []byte) (helperResult, error) {
	var res helperResult
	err := json.Unmarshal(out, &res)
	return res, err
}
