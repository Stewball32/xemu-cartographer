package hooks

import (
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"

	"github.com/Stewball32/xemu-cartographer/internal/halosave"
	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

// attachBundle is the shared core of the generate-on-save hooks. It generates
// the Xbox save artifact for req and writes it onto the record IN PLACE — the
// deployable tar onto the `save_bundle` file field and the SaveSet metadata
// onto `save_info` — before the record persists. Called from the
// OnRecordCreate / OnRecordUpdate hooks (which then call e.Next()), so the file
// lands in the same save transaction without re-firing the hook.
//
// It returns the generator error verbatim so the hook rejects an unbuildable
// record (e.g. an out-of-range appearance byte) rather than storing junk.
func attachBundle(rec *core.Record, req halosave.BuildRequest, filename string) error {
	b, err := saveartifact.Build(req)
	if err != nil {
		return err
	}
	f, err := filesystem.NewFileFromBytes(b.Tar, filename)
	if err != nil {
		return fmt.Errorf("save bundle file: %w", err)
	}
	// Setting a single new *filesystem.File replaces the previous upload; PB
	// reconciles + deletes the orphaned old file on save.
	rec.Set("save_bundle", []*filesystem.File{f})
	rec.Set("save_info", b.Info())
	return nil
}

// slugFilename reduces an arbitrary name to a safe, lowercase filename
// component: alphanumerics kept, every other run collapsed to a single '-'.
// Empty input yields "unnamed".
func slugFilename(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unnamed"
	}
	return out
}
