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

	// snapshotMu guards latestSnapshotMsg, the wrapped websocket.Message bytes
	// for the most recent snapshot broadcast. Replayed to clients that join the
	// overlay room mid-match so the UI can render without waiting for the next
	// game-state transition (M2 follow-up: ROADMAP.md "Snapshot replay for late
	// joiners").
	snapshotMu        sync.Mutex
	latestSnapshotMsg []byte
}

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
