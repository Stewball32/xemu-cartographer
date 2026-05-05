package manager

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// hostSummary is one entry in the host:all aggregate cache. Lean on purpose
// — host:all subscribers are list views (admin debug index, future host
// picker UI), not per-instance overlays. Anything heavier belongs on the
// host:<name> per-instance stream.
type hostSummary struct {
	Instance             string    `json:"instance"`
	Phase                Phase     `json:"phase"`
	Title                string    `json:"title"`
	Map                  string    `json:"map"`
	Gametype             string    `json:"gametype"`
	ScoreSummary         string    `json:"score_summary"`
	LastSuccessfulReadAt time.Time `json:"last_successful_read_at"`
}

// summaryUpdate is one message on the aggregator's input channel. Removed
// distinguishes runner-stop (eviction) from a fresh snapshot push.
type summaryUpdate struct {
	Instance string
	Removed  bool
	Snapshot *hostSummary // nil iff Removed
}

const (
	aggregatorChanBuffer = 64
	// aggregatorCoalesce caps the host:all rebroadcast cadence. 250ms is
	// fast enough that a list view feels live (phase / score updates land
	// within a quarter-second) and slow enough that a 30Hz runner pushing
	// every tick can't fanout 30 messages/sec to every subscriber.
	aggregatorCoalesce = 250 * time.Millisecond
)

// aggregator owns the host:all room: it consumes summaryUpdate events from
// every runner, maintains a deterministic-iteration map of summaries, and
// re-broadcasts the full set on a coalesced cadence (OQ2 — full re-broadcast,
// no diffs). One aggregator per Manager.
//
// Single-goroutine writer per room invariant: runners post via a buffered
// channel; aggregator.run is the only thing that touches hostsCache or
// calls SendToRoomRaw(host:all, ...). On full channel post() drops; since
// hostsCache always converges to "last value wins" the only thing lost is
// intermediate diffs, which is fine for a coalesced list view.
type aggregator struct {
	svc     *guards.Services
	updates chan summaryUpdate
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}

	mu         sync.Mutex
	hostsCache map[string]hostSummary
}

func newAggregator(svc *guards.Services) *aggregator {
	ctx, cancel := context.WithCancel(context.Background())
	return &aggregator{
		svc:        svc,
		updates:    make(chan summaryUpdate, aggregatorChanBuffer),
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		hostsCache: map[string]hostSummary{},
	}
}

// run is the aggregator's goroutine. Started by Manager.New.
func (a *aggregator) run() {
	defer close(a.done)

	ticker := time.NewTicker(aggregatorCoalesce)
	defer ticker.Stop()

	dirty := false
	for {
		select {
		case <-a.ctx.Done():
			return
		case u := <-a.updates:
			a.apply(u)
			dirty = true
		case <-ticker.C:
			if dirty {
				a.broadcast()
				dirty = false
			}
		}
	}
}

// apply mutates hostsCache for one summaryUpdate.
func (a *aggregator) apply(u summaryUpdate) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u.Removed {
		delete(a.hostsCache, u.Instance)
		return
	}
	if u.Snapshot != nil {
		a.hostsCache[u.Instance] = *u.Snapshot
	}
}

// post forwards a summary to the aggregator. Non-blocking; on full channel
// the update is dropped (last-write-wins on the next push for the same
// instance). Safe to call before/after run() exits.
func (a *aggregator) post(u summaryUpdate) {
	if a == nil {
		return
	}
	select {
	case a.updates <- u:
	default:
	}
}

// broadcast renders hostsCache into one envelope and pushes to host:all.
func (a *aggregator) broadcast() {
	if a.svc == nil || a.svc.WS == nil {
		return
	}
	msgBytes, ok := a.marshalEnvelope()
	if !ok {
		return
	}
	a.svc.WS.SendToRoomRaw(rooms.HostAllRoom, msgBytes)
}

// joinReplay returns one envelope-bytes message representing the current
// hostsCache, for replay to clients that just joined host:all. Same shape
// as the broadcast() output.
func (a *aggregator) joinReplay() [][]byte {
	if a == nil {
		return nil
	}
	msgBytes, ok := a.marshalEnvelope()
	if !ok {
		return nil
	}
	return [][]byte{msgBytes}
}

// marshalEnvelope is the shared host:all envelope builder. Returns the
// pre-marshaled wire bytes ready for SendToRoomRaw / SendRaw.
func (a *aggregator) marshalEnvelope() ([]byte, bool) {
	summaries := a.snapshot()
	// "all" as the envelope's Instance field is the client-side disambiguator
	// between host:all summary feed and a per-instance host:<name> stream
	// (both ride the legacy "snapshot" wire type until M5 stage 5c).
	env := scraper.MakeEnvelope(envelopeTypeGameData, "all", 0, summaries)
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("aggregator: marshal envelope: %v", err)
		return nil, false
	}
	msg := websocket.Message{
		Type:    "scraper",
		Room:    rooms.HostAllRoom,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("aggregator: marshal message: %v", err)
		return nil, false
	}
	return msgBytes, true
}

// snapshot returns the cache as a sorted slice (deterministic order).
func (a *aggregator) snapshot() []hostSummary {
	a.mu.Lock()
	out := make([]hostSummary, 0, len(a.hostsCache))
	for _, s := range a.hostsCache {
		out = append(out, s)
	}
	a.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Instance < out[j].Instance })
	return out
}

// stop cancels the goroutine and waits for it to exit.
func (a *aggregator) stop() {
	a.cancel()
	<-a.done
}

// summaryFromCache projects an instanceCache into a hostSummary. Used by
// the runner when posting summary updates. Reads cache fields directly;
// caller must hold cacheMu (or have copied the cache out).
func summaryFromCache(name string, c *instanceCache) hostSummary {
	s := hostSummary{
		Instance:             name,
		Phase:                c.Phase,
		Title:                c.Title,
		LastSuccessfulReadAt: c.LastReadAt,
	}
	if c.GameData != nil {
		s.Map = c.GameData.Map
		s.Gametype = c.GameData.Gametype
		s.ScoreSummary = renderScoreSummary(c.GameData)
	}
	return s
}

// renderScoreSummary builds a compact human-readable score string for the
// host-list UI. Two-team Slayer renders as "12 — 9"; FFA / no-team-data
// returns empty (the UI can fall back to phase / map). Kept minimal; M5e
// can elaborate when the host-list UI lands.
func renderScoreSummary(gd *scraper.GameData) string {
	if gd == nil || len(gd.TeamScores) == 0 {
		return ""
	}
	parts := make([]string, len(gd.TeamScores))
	for i, ts := range gd.TeamScores {
		parts[i] = strconv.FormatInt(int64(ts.Score), 10)
	}
	return strings.Join(parts, " — ")
}
