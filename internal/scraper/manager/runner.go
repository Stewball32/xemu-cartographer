package manager

import (
	"context"
	"sync"
	"time"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// runner holds one scraper goroutine's state. Lifetime: created in
// Manager.Start, owned by Manager.runners[name], destroyed in Manager.Stop
// (which also cancels its context and waits on done).
type runner struct {
	name      string
	sock      string
	titleID   uint32
	inst      *xemu.Instance
	reader    scraper.GameReader
	state     *scraper.TickState
	snapshot  scraper.SnapshotPayload
	startedAt time.Time

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// progress is updated by the loop goroutine; readers (List, info) take
	// progressMu to copy the snapshot.
	progressMu sync.Mutex
	tick       uint32 // most recent observed game tick
	ticks      uint64 // total loop iterations since Start

	// powerItemsInitialised gates state.InitPowerItems so it only fires once
	// per match — and only after the snapshot's PowerItemSpawns carry real
	// InitialObjectIDs (i.e. matchCache.InitialObjIDsFilled = true). Without
	// this gate, InitPowerItems would seed event-detection trackers against
	// 0xFFFF placeholders, then never refresh.
	powerItemsInitialised bool

	// idlePollCount counts non-in_game iterations since the last title-ID
	// re-check, used by the XBE-swap detection (see loop.go). Reset to zero
	// when the title-ID check fires.
	idlePollCount int

	// cacheMu guards every field below: the wrapped websocket.Message bytes
	// for the most recent snapshot broadcast (replayed to overlay-room joiners),
	// plus an unwrapped copy of the latest snapshot/tick/state/events served
	// on-demand by /api/admin/scraper/{name}/inspect. The HTTP handler reads
	// these copies under the lock — it never touches r.reader or r.inst, which
	// the loop goroutine accesses without synchronisation (Reader has
	// unsynchronised tag/biped caches that aren't safe for concurrent reads).
	cacheMu           sync.Mutex
	latestSnapshotMsg []byte
	cachedState       scraper.GameState
	cachedStateInputs scraper.StateInputs
	cachedScoreProbe  scraper.ScoreProbe
	cachedSnapshot    *scraper.SnapshotPayload
	cachedTick        *scraper.TickPayload
	recentEvents      []scraper.Envelope
}

// recentEventsCap caps the per-runner ring buffer surfaced via the inspect
// endpoint. Newest first; older entries drop off the back.
const recentEventsCap = 50

func newRunner(name, sock string, titleID uint32, inst *xemu.Instance, reader scraper.GameReader) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &runner{
		name:      name,
		sock:      sock,
		titleID:   titleID,
		inst:      inst,
		reader:    reader,
		state:     reader.NewTickState(),
		startedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

func (r *runner) info() scraperiface.Info {
	r.progressMu.Lock()
	defer r.progressMu.Unlock()
	return scraperiface.Info{
		Name:      r.name,
		Sock:      r.sock,
		TitleID:   r.titleID,
		GameTitle: r.reader.Title(),
		XboxName:  r.reader.XboxName(),
		Tick:      r.tick,
		Ticks:     r.ticks,
		StartedAt: r.startedAt,
	}
}

func (r *runner) recordTick(tick uint32) {
	r.progressMu.Lock()
	r.tick = tick
	r.ticks++
	r.progressMu.Unlock()
}

func (r *runner) cacheState(gs scraper.GameState) {
	r.cacheMu.Lock()
	r.cachedState = gs
	r.cacheMu.Unlock()
}

func (r *runner) cacheStateInputs(si scraper.StateInputs) {
	if si == nil {
		return
	}
	cp := make(scraper.StateInputs, len(si))
	for k, v := range si {
		cp[k] = v
	}
	r.cacheMu.Lock()
	r.cachedStateInputs = cp
	r.cacheMu.Unlock()
}

func (r *runner) cacheScoreProbe(sp scraper.ScoreProbe) {
	if sp == nil {
		return
	}
	cp := make(scraper.ScoreProbe, len(sp))
	for k, v := range sp {
		cp[k] = v
	}
	r.cacheMu.Lock()
	r.cachedScoreProbe = cp
	r.cacheMu.Unlock()
}

func (r *runner) cacheSnapshot(snap scraper.SnapshotPayload) {
	cp := snap
	r.cacheMu.Lock()
	r.cachedSnapshot = &cp
	r.cacheMu.Unlock()
}

func (r *runner) cacheTick(tp scraper.TickPayload) {
	cp := tp
	r.cacheMu.Lock()
	r.cachedTick = &cp
	r.cacheMu.Unlock()
}

func (r *runner) cacheEvent(env scraper.Envelope) {
	r.cacheMu.Lock()
	r.recentEvents = append([]scraper.Envelope{env}, r.recentEvents...)
	if len(r.recentEvents) > recentEventsCap {
		r.recentEvents = r.recentEvents[:recentEventsCap]
	}
	r.cacheMu.Unlock()
}
