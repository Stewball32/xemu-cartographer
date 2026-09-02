// Package offsets is the version-level data layer for game memory offsets.
//
// A modded build (e.g. "H1 Perf Build") shifts memory addresses without
// changing the game's BEHAVIOR — so the per-game reader/handler code stays
// hardcoded, and only the addresses it dereferences are versioned. Each offset
// SET is a JSON config file in sets/ (embedded at build time), human-readable
// and directly emittable by the halo-offset-mapper (the entry shape matches its
// export format: {"Name": {"value": "0x...", ...}} — extra per-entry fields
// like notes/confidence/source are tolerated and ignored).
//
// Every game has exactly one BASELINE set ("<game>-baseline") holding the
// previously hardcoded constants; a build rides its game's baseline unless the
// catalog row is explicitly pointed at another set (isos.offset_set). Assigning
// a build to an existing set is pure data; authoring a NEW set is a new file in
// sets/ + deploy. Out-of-tree game plugins supply their baseline from init()
// via RegisterBaseline — same JSON format, same parser.
//
// Consumers: scraper.Detect resolves the set for the instance and hands it to
// the game plugin's factory, which binds it into a typed per-game struct
// (haloce.OffsetsFromSet / halo2.OffsetsFromSet) — missing keys fail loudly at
// bind time, never mid-read.
package offsets

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed sets/*.json
var setsFS embed.FS

// entry is one named value in a set file. Only value/type are read; the mapper
// export's extra fields (kind/confidence/notes/source) are ignored.
type entry struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type setFile struct {
	Game        string           `json:"game"`
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Offsets     map[string]entry `json:"offsets"`
}

// Set is one parsed, immutable offset set.
type Set struct {
	Game        string
	ID          string
	Description string

	addrs   map[string]uint32
	strings map[string]string
}

// Addr returns the named address/offset value. Errors on a missing key or a
// key that holds a string — binding code turns this into a load-time failure.
func (s *Set) Addr(name string) (uint32, error) {
	if v, ok := s.addrs[name]; ok {
		return v, nil
	}
	if _, ok := s.strings[name]; ok {
		return 0, fmt.Errorf("offset set %s/%s: %q is a string, not an address", s.Game, s.ID, name)
	}
	return 0, fmt.Errorf("offset set %s/%s: missing offset %q", s.Game, s.ID, name)
}

// Str returns the named string value (e.g. a UI tag path).
func (s *Set) Str(name string) (string, error) {
	if v, ok := s.strings[name]; ok {
		return v, nil
	}
	return "", fmt.Errorf("offset set %s/%s: missing string offset %q", s.Game, s.ID, name)
}

// Len reports how many named values the set carries.
func (s *Set) Len() int { return len(s.addrs) + len(s.strings) }

var (
	registry    = map[string]*Set{}   // key: id (ids are globally unique)
	rawRegistry = map[string][]byte{} // embedded/registered file bytes, for re-export
	rawNames    = map[string]string{} // source filename per id
	baselineIDs = map[string]string{} // game key → RegisterBaseline'd set id
)

func init() {
	entries, err := setsFS.ReadDir("sets")
	if err != nil {
		panic(fmt.Sprintf("offsets: read embedded sets: %v", err))
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := setsFS.ReadFile("sets/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("offsets: read %s: %v", e.Name(), err))
		}
		s, err := parseSet(raw)
		if err != nil {
			panic(fmt.Sprintf("offsets: parse %s: %v", e.Name(), err))
		}
		if _, dup := registry[s.ID]; dup {
			panic(fmt.Sprintf("offsets: duplicate set id %q", s.ID))
		}
		registry[s.ID] = s
		rawRegistry[s.ID] = raw
		rawNames[s.ID] = e.Name()
	}
}

// Raw returns a compiled-in set's original file bytes + source filename (for
// the download endpoint — byte-identical to what shipped, whether embedded in
// this package or registered by a plugin). ok=false for ids that aren't
// compiled in (imported sets serve their stored upload instead).
func Raw(id string) (raw []byte, sourceName string, ok bool) {
	raw, ok = rawRegistry[id]
	return raw, rawNames[id], ok
}

func parseSet(raw []byte) (*Set, error) {
	var f setFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if f.Game == "" || f.ID == "" {
		return nil, fmt.Errorf("set file must carry non-empty game + id")
	}
	s := &Set{
		Game:        f.Game,
		ID:          f.ID,
		Description: f.Description,
		addrs:       map[string]uint32{},
		strings:     map[string]string{},
	}
	for name, en := range f.Offsets {
		if en.Type == "string" {
			s.strings[name] = en.Value
			continue
		}
		v, err := strconv.ParseUint(strings.TrimPrefix(en.Value, "0x"), 16, 32)
		if err != nil {
			return nil, fmt.Errorf("offset %q: bad hex value %q: %w", name, en.Value, err)
		}
		s.addrs[name] = uint32(v)
	}
	return s, nil
}

// BaselineID is the baseline set id for a game key: a RegisterBaseline'd id
// when the game's plugin supplied one, else the conventional "<game>-baseline"
// (with the legacy short ids for the embedded games).
func BaselineID(game string) string {
	if id, ok := baselineIDs[game]; ok {
		return id
	}
	switch game {
	case "haloce":
		return "ce-baseline"
	case "halo2":
		return "h2-baseline"
	}
	return game + "-baseline"
}

// Baseline returns the baseline set for a game key, erroring when the game has
// none — callers degrade (a runner stays idle) rather than die.
func Baseline(game string) (*Set, error) {
	s, ok := registry[BaselineID(game)]
	if !ok || s.Game != game {
		return nil, fmt.Errorf("offsets: no baseline set for game %q", game)
	}
	return s, nil
}

// RegisterBaseline registers a game's baseline set from raw offsetmap JSON.
// Out-of-tree game plugins call this from init(), before any scraper binds:
// the registry maps are unsynchronised, so mutate them only during package
// initialisation (same rule as SetDynamicSource). Embedded sets already cover
// the in-tree games. The set's game must match the given key, its id must be
// globally unique, and a game gets exactly one baseline.
func RegisterBaseline(game string, raw []byte) error {
	s, err := parseSet(raw)
	if err != nil {
		return fmt.Errorf("offsets: register baseline for %q: %w", game, err)
	}
	if s.Game != game {
		return fmt.Errorf("offsets: register baseline for %q: set %q belongs to game %q", game, s.ID, s.Game)
	}
	if _, dup := registry[s.ID]; dup {
		return fmt.Errorf("offsets: register baseline for %q: duplicate set id %q", game, s.ID)
	}
	if b, err := Baseline(game); err == nil {
		return fmt.Errorf("offsets: game %q already has baseline %q", game, b.ID)
	}
	registry[s.ID] = s
	rawRegistry[s.ID] = raw
	rawNames[s.ID] = s.ID + ".json"
	baselineIDs[game] = s.ID
	return nil
}

// DynamicSource supplies runtime-imported sets by id (the PB-backed
// offset_sets collection, wired from main.go). Consulted only when an id
// isn't in the embedded registry; returning ok=false means "no such set".
// The raw bytes are parsed + validated exactly like an embedded file.
type DynamicSource func(id string) (raw []byte, ok bool)

var dynamic DynamicSource

// SetDynamicSource installs the runtime set source. Call once at startup,
// before any scraper binds (later calls replace the source).
func SetDynamicSource(fn DynamicSource) { dynamic = fn }

// ParseSet parses one offsetmap JSON export into an immutable Set. Exposed for
// the import route (server-side re-validation of an upload) and the dynamic
// source path; embedded files go through the same parser at init.
func ParseSet(raw []byte) (*Set, error) { return parseSet(raw) }

// Lookup returns the set with the given id, requiring it to belong to `game`.
// Embedded sets win; unknown ids fall through to the dynamic source.
func Lookup(game, id string) (*Set, error) {
	s, ok := registry[id]
	if !ok && dynamic != nil {
		if raw, found := dynamic(id); found {
			parsed, err := parseSet(raw)
			if err != nil {
				return nil, fmt.Errorf("offsets: imported set %q: %w", id, err)
			}
			// The record's set_id is the identity ("Save as" may have renamed the
			// import); the stored file stays byte-identical for re-download, so
			// its internal id can lag — the record wins.
			parsed.ID = id
			s, ok = parsed, true
		}
	}
	if !ok {
		return nil, fmt.Errorf("offsets: unknown set id %q", id)
	}
	if s.Game != game {
		return nil, fmt.Errorf("offsets: set %q belongs to game %q, not %q", id, s.Game, game)
	}
	return s, nil
}

// Resolve picks the set an instance should run: the explicit id when given and
// valid for the game, else the game's baseline. An invalid explicit id degrades
// to the baseline with a non-nil set AND a non-nil warning (fail-soft — a bad
// data assignment must not stop a vanilla box from booting). A nil set means
// the game has no baseline to fall back on; the error is then fatal.
func Resolve(game, explicitID string) (s *Set, warn error) {
	if explicitID == "" {
		return Baseline(game)
	}
	s, err := Lookup(game, explicitID)
	if err != nil {
		base, baseErr := Baseline(game)
		if baseErr != nil {
			return nil, fmt.Errorf("%v; %w", err, baseErr)
		}
		return base, fmt.Errorf("falling back to %s: %w", BaselineID(game), err)
	}
	return s, nil
}

// SetInfo describes one registered set (for the admin listing endpoint).
type SetInfo struct {
	Game        string `json:"game"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Count       int    `json:"count"`
	Baseline    bool   `json:"baseline"`
}

// All lists every registered set, sorted by game then id.
func All() []SetInfo {
	out := make([]SetInfo, 0, len(registry))
	for _, s := range registry {
		out = append(out, SetInfo{
			Game:        s.Game,
			ID:          s.ID,
			Description: s.Description,
			Count:       s.Len(),
			Baseline:    s.ID == BaselineID(s.Game),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Game != out[j].Game {
			return out[i].Game < out[j].Game
		}
		return out[i].ID < out[j].ID
	})
	return out
}
