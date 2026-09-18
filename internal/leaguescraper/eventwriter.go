package leaguescraper

import (
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/capture"
	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"
)

// EventWriter is the wire-mode home of the pb: capture sinks (DESIGN-STEP8
// §10, replacing the runner-side PBSink of pbsink.go): every scraper frame
// the league receives from the daemon is resolved against the PB
// capture_policies rows (capture.Resolve) and, when the winning row's sink
// is pb:<collection>, written as a game_events-style record exactly as the
// embedded sink did. Rows on demand-gated classes whose mode demands reads
// (mode always) add host:<inst>:<class> to the upstream demand set so the
// daemon keeps reading without a league WS subscriber — the
// runner/demand.go shouldRead semantics (hard cap → nothing; always → read;
// otherwise subscribers decide) carried across the wire.
//
// It satisfies Pusher, so the existing capture-policy provider
// (ReloadCapturePolicies + RegisterCapturePolicyHooks) feeds it the rows.
type EventWriter struct {
	app  core.App
	opt  EventWriterOptions
	logf func(string, ...any)

	mu       sync.Mutex
	policies []capture.Policy
	sinks    map[string]*PBSink
	joined   map[string]bool

	written atomic.Uint64
	failed  atomic.Uint64
}

// DemandPort is the upstream demand set (*xcclient.Client satisfies it). It
// is refcounted per room and shared with the demand observer (downstream
// viewers): the writer holds exactly one reference per room it wants
// (Resync keeps Join/Leave balanced), so a viewer's linger leave cannot
// drop a room the sink still needs and vice versa.
type DemandPort interface {
	Join(room string)
	Leave(room string)
}

// EventWriterOptions configures NewEventWriter.
type EventWriterOptions struct {
	// Demand receives Join/Leave for sink-driven rooms; nil ⇒ no demand.
	Demand DemandPort
	// Instances lists the known instance names (e.g. the mirror's) so
	// wildcard-instance rows expand; nil ⇒ only explicitly named instances.
	Instances func() []string
	Logf      func(format string, args ...any)
}

// NewEventWriter returns a writer with no policies (nothing written, no
// demand) until SetCapturePolicies runs.
func NewEventWriter(app core.App, opt EventWriterOptions) *EventWriter {
	logf := opt.Logf
	if logf == nil {
		logf = log.Printf
	}
	return &EventWriter{app: app, opt: opt, logf: logf, sinks: map[string]*PBSink{}, joined: map[string]bool{}}
}

// Written / Failed count sink writes.
func (w *EventWriter) Written() uint64 { return w.written.Load() }
func (w *EventWriter) Failed() uint64  { return w.failed.Load() }

// SetCapturePolicies installs the PB rows (Pusher) and resyncs demand.
func (w *EventWriter) SetCapturePolicies(policies []capture.Policy) {
	w.mu.Lock()
	w.policies = append([]capture.Policy(nil), policies...)
	w.mu.Unlock()
	w.Resync()
}

// CapturePolicies returns a copy of the installed rows.
func (w *EventWriter) CapturePolicies() []capture.Policy {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]capture.Policy(nil), w.policies...)
}

// Rooms returns the sink-driven rooms currently joined upstream (sorted).
func (w *EventWriter) Rooms() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.joined))
	for r := range w.joined {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// OnFrame is the xcclient.Config.OnFrame half for persistence: per-instance
// class frames are resolved and written; hello + summary frames (the
// instance set may have changed) trigger a demand resync.
func (w *EventWriter) OnFrame(f xcclient.Frame) {
	if f.Env == nil {
		return
	}
	switch {
	case f.Env.Type == wire.ClassHello || f.Env.Type == wire.ClassSummary:
		w.Resync()
		return
	case !wire.IsPerInstanceClass(f.Env.Type) || f.Env.Instance == "":
		return
	}
	w.mu.Lock()
	pol := capture.Resolve(w.policies, f.Env.Instance, f.Env.Type)
	w.mu.Unlock()
	if pol.IsHardCap() || !IsPBSink(pol.Sink) {
		return
	}
	if err := w.sink(pol.Sink).Write(f.Msg.Payload); err != nil {
		w.failed.Add(1)
		w.logf("eventwriter: %s/%s → %s: %v", f.Env.Instance, f.Env.Type, pol.Sink, err)
		return
	}
	w.written.Add(1)
}

func (w *EventWriter) sink(spec string) *PBSink {
	coll := strings.TrimPrefix(spec, PBSinkPrefix)
	w.mu.Lock()
	defer w.mu.Unlock()
	s, ok := w.sinks[coll]
	if !ok {
		s = NewPBSink(w.app, coll)
		w.sinks[coll] = s
	}
	return s
}

// Resync recomputes the sink-driven demand rooms from the policies and the
// known instances and reconciles the upstream demand set (Join new rooms,
// Leave stale ones).
func (w *EventWriter) Resync() {
	w.mu.Lock()
	wanted := demandRooms(w.policies, w.instanceNames())
	var join, leave []string
	for room := range wanted {
		if !w.joined[room] {
			join = append(join, room)
		}
	}
	for room := range w.joined {
		if !wanted[room] {
			leave = append(leave, room)
		}
	}
	w.joined = wanted
	w.mu.Unlock()
	if w.opt.Demand == nil {
		return
	}
	sort.Strings(join)
	sort.Strings(leave)
	for _, room := range leave {
		w.opt.Demand.Leave(room)
	}
	for _, room := range join {
		w.opt.Demand.Join(room)
	}
}

// instanceNames unions the Instances source with every instance named
// explicitly by a row (a named row joins before the box appears, exactly
// like a policy row on a not-yet-attached runner). Caller holds w.mu.
func (w *EventWriter) instanceNames() []string {
	seen := map[string]bool{}
	var names []string
	add := func(n string) {
		if n == "" || n == capture.Wildcard || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	if w.opt.Instances != nil {
		for _, n := range w.opt.Instances() {
			add(n)
		}
	}
	for _, p := range w.policies {
		add(p.Instance)
	}
	return names
}

// demandRooms is the pure core of Resync: for every (instance, demand-gated
// class) whose resolved policy has a pb: sink and demands reads, the
// per-class room.
func demandRooms(policies []capture.Policy, instances []string) map[string]bool {
	out := map[string]bool{}
	for _, inst := range instances {
		for _, class := range xcclient.DemandClasses {
			pol := capture.Resolve(policies, inst, class)
			if !IsPBSink(pol.Sink) || !pol.Demands() {
				continue
			}
			room, err := wire.RoomForInstanceClass(inst, class)
			if err != nil {
				continue
			}
			out[room] = true
		}
	}
	return out
}
