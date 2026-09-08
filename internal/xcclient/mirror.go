package xcclient

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/xemu-cartographer/xc-scraper/membership"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// Mirror is the league-side copy of the daemon's hot state (DESIGN-STEP8 §8.2):
// per instance the daemon's started_at, the raw last frame per class (bytes
// untouched — room and seq preserved, so join replays are byte-identical to
// what the daemon sent) and the decoded xbox / game / previous_game payloads
// the league reads synchronously (List, InstanceState, Membership). The
// stream worker (stream.go) is the only writer; every reader takes the
// read lock. All exported reads are safe for concurrent use.
type Mirror struct {
	mu      sync.RWMutex
	insts   map[string]*mirrorInstance
	summary []byte
	hosts   *wire.SummaryPayload
}

type mirrorInstance struct {
	startedAt   time.Time
	placeholder bool // startedAt is an envelope ts stand-in (GET /api/instances failed)
	frames      map[string][]byte
	seq         map[string]uint64
	tick        uint32
	xbox        *wire.XboxPayload
	game        *wire.GamePayload
	prev        *wire.PreviousGamePayload
	identities  []string
}

// StoreResult reports what Store did with a frame so the stream worker can
// act on epoch changes (Regression) and instance discovery (New).
type StoreResult struct {
	New        bool   // the instance was unknown; created with a placeholder started_at
	Regression bool   // seq went backwards: the instance's cache was cleared before storing
	Gap        uint64 // frames skipped since the last seq (0 = contiguous or untracked)
	Cached     bool   // the frame is now the instance's cached frame for its class
}

// NewMirror returns an empty mirror.
func NewMirror() *Mirror {
	return &Mirror{insts: make(map[string]*mirrorInstance)}
}

func newMirrorInstance(startedAt time.Time, placeholder bool) *mirrorInstance {
	return &mirrorInstance{
		startedAt:   startedAt,
		placeholder: placeholder,
		frames:      make(map[string][]byte),
		seq:         make(map[string]uint64),
		identities:  membership.Identities("", nil, nil),
	}
}

// SetInstance records an instance with the daemon's started_at. A new
// instance is added (changed=false). An existing instance whose real
// started_at differs is a new epoch: its cache is cleared and changed=true.
// Replacing a placeholder with the real value (or a placeholder with another
// placeholder) never counts as an epoch change.
func (m *Mirror) SetInstance(name string, startedAt time.Time, placeholder bool) (changed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mi, ok := m.insts[name]
	if !ok {
		m.insts[name] = newMirrorInstance(startedAt, placeholder)
		return false
	}
	if placeholder {
		if mi.placeholder {
			mi.startedAt = startedAt
		}
		return false
	}
	if mi.placeholder || mi.startedAt.Equal(startedAt) {
		mi.startedAt = startedAt
		mi.placeholder = false
		return false
	}
	mi.startedAt = startedAt
	mi.placeholder = false
	mi.reset()
	return true
}

// Remove forgets an instance entirely. Returns false when it was unknown.
func (m *Mirror) Remove(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.insts[name]; !ok {
		return false
	}
	delete(m.insts, name)
	return true
}

// ClearInstance drops an instance's cached frames, seq state and decoded
// payloads but keeps it (and its started_at) in the instance set.
func (m *Mirror) ClearInstance(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := m.insts[name]; ok {
		mi.reset()
	}
}

// MarkPlaceholder flags an instance's started_at as provisional (kept as
// is) so the next successful GET /api/instances read replaces it without
// counting as an epoch change — used after a seq regression, when the
// daemon-side runner restarted and the real value is not known yet.
func (m *Mirror) MarkPlaceholder(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := m.insts[name]; ok {
		mi.placeholder = true
	}
}

// Clear empties the mirror (stale upstream, §8.2).
func (m *Mirror) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insts = make(map[string]*mirrorInstance)
	m.summary = nil
	m.hosts = nil
}

// Drop forgets the cached frame of one (instance, class) — the demand layer
// calls it when it leaves a demand-gated room so a late joiner is served by
// the upstream replay instead of a frozen frame.
func (m *Mirror) Drop(name, class string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mi, ok := m.insts[name]; ok {
		delete(mi.frames, class)
		delete(mi.seq, class)
	}
}

func (mi *mirrorInstance) reset() {
	mi.frames = make(map[string][]byte)
	mi.seq = make(map[string]uint64)
	mi.tick = 0
	mi.xbox = nil
	mi.game = nil
	mi.prev = nil
	mi.identities = membership.Identities("", nil, nil)
}

// Store records a per-instance frame: raw is the framed wire.Message exactly
// as received, env its decoded envelope. Unknown instances are created with
// env.Ts as a placeholder started_at (New=true). Seq is tracked for state
// classes only (events always carry seq 0); a regression clears the
// instance first. Only wire.StateClasses frames are cached — events are a
// log, not state, and are never replayed on join.
func (m *Mirror) Store(name, class string, raw []byte, env *wire.Envelope) StoreResult {
	var res StoreResult
	if name == "" || !stateClassSet[class] {
		return res
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mi, ok := m.insts[name]
	if !ok {
		mi = newMirrorInstance(env.Ts, true)
		m.insts[name] = mi
		res.New = true
	}
	if seqTracked(class) {
		if last, seen := mi.seq[class]; seen {
			switch {
			case env.Seq < last:
				mi.reset()
				res.Regression = true
			case env.Seq > last+1:
				res.Gap = env.Seq - last - 1
			}
		}
		mi.seq[class] = env.Seq
	}
	mi.frames[class] = raw
	res.Cached = true
	if env.Tick > 0 {
		mi.tick = env.Tick
	}
	mi.decode(class, env.Data)
	return res
}

// stateClassSet is wire.StateClasses as a set: the classes with a "last
// frame" worth caching (events are a log, never replayed on join).
var stateClassSet = func() map[string]bool {
	set := make(map[string]bool)
	for _, class := range wire.StateClasses() {
		set[class] = true
	}
	return set
}()

// seqTracked reports whether a class carries a meaningful per-(instance,
// class) seq (wire.md "About seq": events always ship seq 0).
func seqTracked(class string) bool {
	switch class {
	case wire.ClassEvent, wire.ClassEventFiltered:
		return false
	}
	return true
}

func (mi *mirrorInstance) decode(class string, data json.RawMessage) {
	switch class {
	case wire.ClassXbox:
		var p wire.XboxPayload
		if json.Unmarshal(data, &p) == nil {
			mi.xbox = &p
			mi.rebuildIdentities()
		}
	case wire.ClassGame:
		var p wire.GamePayload
		if json.Unmarshal(data, &p) == nil {
			mi.game = &p
			mi.rebuildIdentities()
		}
	case wire.ClassPreviousGame:
		var p wire.PreviousGamePayload
		if json.Unmarshal(data, &p) == nil {
			mi.prev = &p
		}
	}
}

func (mi *mirrorInstance) rebuildIdentities() {
	xboxName := ""
	if mi.xbox != nil {
		xboxName = mi.xbox.Name
	}
	mi.identities = membership.FromGamePayload(xboxName, mi.game)
}

// StoreSummary caches the raw summary frame and its decoded hosts.
func (m *Mirror) StoreSummary(raw []byte, payload *wire.SummaryPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.summary = raw
	m.hosts = payload
}

// Placeholders lists the instances whose started_at is still an envelope
// ts stand-in (sorted).
func (m *Mirror) Placeholders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	for name, mi := range m.insts {
		if mi.placeholder {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Instances returns the known instance names sorted (runner.Manager.List order).
func (m *Mirror) Instances() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.namesLocked()
}

func (m *Mirror) namesLocked() []string {
	names := make([]string, 0, len(m.insts))
	for name := range m.insts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// StartedAt returns an instance's started_at (false when unknown).
func (m *Mirror) StartedAt(name string) (time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mi, ok := m.insts[name]
	if !ok {
		return time.Time{}, false
	}
	return mi.startedAt, true
}

// List projects the mirror onto []runner.Info the way Manager.List does:
// Name, StartedAt, Tick and the xbox-class identity fields; Sock is "" and
// Ticks / BoundOffsetSet are not on the wire.
func (m *Mirror) List() []runner.Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := m.namesLocked()
	out := make([]runner.Info, 0, len(names))
	for _, name := range names {
		out = append(out, m.insts[name].info(name))
	}
	return out
}

func (mi *mirrorInstance) info(name string) runner.Info {
	info := runner.Info{Name: name, StartedAt: mi.startedAt, Tick: mi.tick}
	x := mi.xbox
	if x == nil {
		return info
	}
	info.TitleID = x.TitleID
	info.Title = x.Title
	info.XboxName = x.Name
	info.SerialNumber = x.SerialNumber
	info.MACAddress = x.MACAddress
	info.VideoStandard = x.VideoStandard
	if tz := x.TimeZone; tz != nil {
		info.TimeZoneBias = tz.BiasMinutes
		info.TimeZoneStdName = tz.StdName
		info.TimeZoneDltName = tz.DltName
	}
	if xbe := x.XBE; xbe != nil {
		info.XBETitleName = xbe.TitleName
		info.XBEVersion = xbe.Version
		info.XBEGameRegion = xbe.GameRegion
		info.XBEDiskNumber = xbe.DiskNumber
		info.XBEAllowedMedia = xbe.AllowedMedia
	}
	if k := x.Kernel; k != nil {
		info.KernelSystemTime = k.SystemTime
		info.KernelBootTime = k.BootTime
		info.KernelUptime = time.Duration(k.UptimeSeconds * float64(time.Second))
	}
	return info
}

// InstanceState mirrors Manager.InstanceState: title / Xbox name from the
// xbox class; Running is true for every instance the daemon reports.
func (m *Mirror) InstanceState(name string) (runner.InstanceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mi, ok := m.insts[name]
	if !ok {
		return runner.InstanceState{Name: name}, false
	}
	st := runner.InstanceState{Name: name, Running: true}
	if x := mi.xbox; x != nil {
		st.TitleID = x.TitleID
		st.Title = x.Title
		st.XboxName = x.Name
	}
	return st, true
}

// Membership returns the per-container identity sets (sorted by container),
// rebuilt from the xbox + game classes via membership.FromGamePayload.
func (m *Mirror) Membership() []membership.ContainerMembership {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := m.namesLocked()
	out := make([]membership.ContainerMembership, 0, len(names))
	for _, name := range names {
		out = append(out, membership.ContainerMembership{
			Container:  name,
			Identities: append([]string(nil), m.insts[name].identities...),
		})
	}
	return out
}

// Xbox returns the decoded xbox payload (nil when not cached). Read-only.
func (m *Mirror) Xbox(name string) *wire.XboxPayload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mi, ok := m.insts[name]; ok {
		return mi.xbox
	}
	return nil
}

// Game returns the decoded game payload (nil when not cached). Read-only.
func (m *Mirror) Game(name string) *wire.GamePayload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mi, ok := m.insts[name]; ok {
		return mi.game
	}
	return nil
}

// PreviousGame returns the decoded previous_game payload (D-7 detector).
func (m *Mirror) PreviousGame(name string) *wire.PreviousGamePayload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mi, ok := m.insts[name]; ok {
		return mi.prev
	}
	return nil
}

// Summary returns the decoded hosts of the last summary frame.
func (m *Mirror) Summary() (wire.SummaryPayload, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.hosts == nil {
		return wire.SummaryPayload{}, false
	}
	return *m.hosts, true
}

// Frame returns the cached raw frame of one (instance, class), nil if none.
func (m *Mirror) Frame(name, class string) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if mi, ok := m.insts[name]; ok {
		return mi.frames[class]
	}
	return nil
}

// JoinReplayMessages is every instance (Instances order) × wire.StateClasses
// order, cached frames only — the scraperiface.JoinReplay shape.
func (m *Mirror) JoinReplayMessages() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out [][]byte
	for _, name := range m.namesLocked() {
		out = append(out, m.insts[name].stateFrames()...)
	}
	return out
}

// JoinReplayForInstance is the bare host:<inst> replay: every cached state
// class in wire.StateClasses order.
func (m *Mirror) JoinReplayForInstance(name string) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mi, ok := m.insts[name]
	if !ok {
		return nil
	}
	return mi.stateFrames()
}

// JoinReplayForInstanceClass is the host:<inst>:<class> replay.
func (m *Mirror) JoinReplayForInstanceClass(name, class string) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	mi, ok := m.insts[name]
	if !ok {
		return nil
	}
	if raw, ok := mi.frames[class]; ok {
		return [][]byte{raw}
	}
	return nil
}

// JoinReplayForHostAll is the host:all / host:summary replay: the cached
// summary frame (room host:summary, as the daemon sent it).
func (m *Mirror) JoinReplayForHostAll() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.summary == nil {
		return nil
	}
	return [][]byte{m.summary}
}

func (mi *mirrorInstance) stateFrames() [][]byte {
	var out [][]byte
	for _, class := range wire.StateClasses() {
		if raw, ok := mi.frames[class]; ok {
			out = append(out, raw)
		}
	}
	return out
}
