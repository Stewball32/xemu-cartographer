// Package manager owns the per-instance scraper lifecycle: it opens an
// xemu.Instance, runs scraper.Detect to pick a game-specific GameReader,
// runs a tick goroutine that broadcasts snapshot/tick/event envelopes
// to the WebSocket "overlay" room, and tears everything down on Stop.
//
// One Manager per server. Routes (/api/admin/scraper/*) and the discovery
// watcher (internal/discovery → onAdd) call Start/Stop/List through the
// scraperiface.Service interface.
package manager

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// ErrAlreadyRunning is returned from Start when name is already in use.
var ErrAlreadyRunning = errors.New("scraper already running")

// Manager owns a name → runner map and dispatches lifecycle operations.
// Implements scraperiface.Service via structural typing.
type Manager struct {
	svc *guards.Services

	mu      sync.Mutex
	runners map[string]*runner
}

// New constructs a Manager that broadcasts via svc.WS. svc may be nil for
// tests; in that case broadcasts become no-ops.
func New(svc *guards.Services) *Manager {
	return &Manager{
		svc:     svc,
		runners: make(map[string]*runner),
	}
}

// Start spins up a scraper for the named instance. It opens the xemu instance
// at sock, runs game detection, and spawns the tick goroutine. Returns
// ErrAlreadyRunning if name is already in use, or whatever error xemu.Init or
// scraper.Detect surfaced (unknown title IDs land here).
func (m *Manager) Start(name, sock string) error {
	if name == "" {
		return errors.New("scraper: name required")
	}
	if sock == "" {
		return errors.New("scraper: sock required")
	}

	m.mu.Lock()
	if _, exists := m.runners[name]; exists {
		m.mu.Unlock()
		return ErrAlreadyRunning
	}
	m.mu.Unlock()

	inst := &xemu.Instance{Name: name, QMPSock: sock}
	if err := inst.Init(scraper.DetectionGVAs()); err != nil {
		return fmt.Errorf("scraper: init xemu instance: %w", err)
	}

	reader, titleID, err := scraper.Detect(inst, name)
	if err != nil {
		inst.Close()
		return fmt.Errorf("scraper: detect game: %w", err)
	}

	// Re-init with the union of detection GVAs and the game's required low GVAs
	// so all per-game pointer globals are pre-translated. xemu.Instance.Init is
	// idempotent for already-translated addresses (cached lookup).
	allGVAs := append(scraper.DetectionGVAs(), reader.LowGVAs()...)
	if err := inst.Init(allGVAs); err != nil {
		inst.Close()
		return fmt.Errorf("scraper: init game low GVAs: %w", err)
	}

	r := newRunner(name, sock, titleID, inst, reader)

	m.mu.Lock()
	// Re-check under lock — guards against two concurrent Start calls racing.
	if _, exists := m.runners[name]; exists {
		m.mu.Unlock()
		r.cancel()
		inst.Close()
		return ErrAlreadyRunning
	}
	m.runners[name] = r
	m.mu.Unlock()

	go r.loop(m.svc)
	return nil
}

// Stop cancels the named runner's context, closes its xemu.Instance, and
// removes it from the registry. Returns nil if name is unknown (idempotent).
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	r, ok := m.runners[name]
	if ok {
		delete(m.runners, name)
	}
	m.mu.Unlock()

	if !ok {
		return nil
	}
	r.cancel()
	<-r.done
	return nil
}

// List returns one Info per currently-tracked runner. Sorted by name for
// stable output.
func (m *Manager) List() []scraperiface.Info {
	m.mu.Lock()
	infos := make([]scraperiface.Info, 0, len(m.runners))
	for _, r := range m.runners {
		infos = append(infos, r.info())
	}
	m.mu.Unlock()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// InstanceState returns a per-runner view (game title, Xbox name) for the
// container detail page. Returns (zero, false) when no runner is attached.
func (m *Manager) InstanceState(name string) (scraperiface.InstanceState, bool) {
	m.mu.Lock()
	r, ok := m.runners[name]
	m.mu.Unlock()
	if !ok {
		return scraperiface.InstanceState{Name: name}, false
	}
	return scraperiface.InstanceState{
		Name:      name,
		TitleID:   r.titleID,
		GameTitle: r.reader.Title(),
		XboxName:  r.reader.XboxName(),
		Running:   true,
	}, true
}

// LatestSnapshotMessages returns the most recent wrapped websocket.Message
// bytes for every runner that has emitted at least one snapshot. Used by the
// join_room handler to replay snapshots to clients joining the overlay room
// mid-match — without this, late joiners only see ticks/events going forward
// and the overlay UI never gets map/players/power-item-spawn data to render.
//
// Each returned []byte is a copy; callers can hold or send it without locking.
func (m *Manager) LatestSnapshotMessages() [][]byte {
	m.mu.Lock()
	runners := make([]*runner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.Unlock()

	out := make([][]byte, 0, len(runners))
	for _, r := range runners {
		r.snapshotMu.Lock()
		if len(r.latestSnapshotMsg) > 0 {
			buf := make([]byte, len(r.latestSnapshotMsg))
			copy(buf, r.latestSnapshotMsg)
			out = append(out, buf)
		}
		r.snapshotMu.Unlock()
	}
	return out
}
