package leaguescraper

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/capture"
	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"
)

// recDemand records Join/Leave calls (DemandPort).
type recDemand struct {
	mu    sync.Mutex
	calls []string
}

func (d *recDemand) Join(room string)  { d.add("join " + room) }
func (d *recDemand) Leave(room string) { d.add("leave " + room) }
func (d *recDemand) add(s string) {
	d.mu.Lock()
	d.calls = append(d.calls, s)
	d.mu.Unlock()
}
func (d *recDemand) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.calls
	d.calls = nil
	return out
}

func ensureEventsCollection(t *testing.T, app core.App) {
	t.Helper()
	ensureCollection(t, app, "game_events",
		&core.TextField{Name: "instance", Required: true},
		&core.TextField{Name: "type", Required: true},
		&core.NumberField{Name: "seq", OnlyInt: true},
		&core.NumberField{Name: "tick", OnlyInt: true},
		&core.DateField{Name: "ts"},
		&core.JSONField{Name: "data", MaxSize: 1 << 20},
	)
}

// frame builds a scraper frame the way the xcclient hands them to OnFrame.
func frame(t *testing.T, instance, class string, seq uint64) xcclient.Frame {
	t.Helper()
	env := wire.Envelope{V: 2, Type: class, Instance: instance, Seq: seq, Tick: 7, Ts: time.Unix(1_700_000_000, 0).UTC(), Data: json.RawMessage(`{"k":1}`)}
	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	msg := wire.Message{Type: wire.TypeScraper, Room: "host:" + instance + ":" + class, Payload: payload}
	raw, _ := json.Marshal(msg)
	return xcclient.Frame{Raw: raw, Msg: msg, Env: &env}
}

// TestEventWriterWritesPBSinkRows: a frame whose resolved policy carries a
// pb: sink lands as a game_events record (the PBSink write); frames without
// a pb: row, hard-capped rows, non-scraper and aggregate frames are ignored.
func TestEventWriterWritesPBSinkRows(t *testing.T) {
	app := newPolicyApp(t)
	ensureEventsCollection(t, app)
	w := NewEventWriter(app, EventWriterOptions{Logf: t.Logf})
	var _ Pusher = w

	w.OnFrame(frame(t, "alpha", wire.ClassEvent, 1)) // no policies yet
	w.SetCapturePolicies([]capture.Policy{
		{Instance: "alpha", Class: wire.ClassEvent, Mode: capture.ModeAlways, Sink: "pb:game_events"},
		{Instance: "beta", Class: wire.ClassEvent, Mode: capture.ModeNever, Sink: "pb:game_events"},
		{Instance: "*", Class: wire.ClassTick, Mode: capture.ModeAuto, Sink: "file:/tmp/t.ndjson"},
	})
	w.OnFrame(frame(t, "alpha", wire.ClassEvent, 2))
	w.OnFrame(frame(t, "alpha", wire.ClassEvent, 3))
	w.OnFrame(frame(t, "beta", wire.ClassEvent, 1))  // hard cap
	w.OnFrame(frame(t, "gamma", wire.ClassEvent, 1)) // no row
	w.OnFrame(frame(t, "alpha", wire.ClassTick, 1))  // non-pb sink
	w.OnFrame(xcclient.Frame{Raw: []byte(`{"type":"host_runner"}`)})
	w.OnFrame(frame(t, "", wire.ClassSummary, 1))
	w.OnFrame(frame(t, "alpha", wire.ClassHello, 1))

	if w.Written() != 2 || w.Failed() != 0 {
		t.Fatalf("written=%d failed=%d", w.Written(), w.Failed())
	}
	rows, err := app.FindAllRecords("game_events")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	seqs := map[float64]bool{}
	for _, r := range rows {
		if r.GetString("instance") != "alpha" || r.GetString("type") != wire.ClassEvent || r.GetInt("tick") != 7 {
			t.Fatalf("row = %v", r.PublicExport())
		}
		seqs[r.GetFloat("seq")] = true
	}
	if !seqs[2] || !seqs[3] {
		t.Fatalf("seqs = %v", seqs)
	}

	// A missing collection is a logged failure, not a panic.
	w.SetCapturePolicies([]capture.Policy{{Instance: "alpha", Class: wire.ClassEvent, Mode: capture.ModeAlways, Sink: "pb:nope"}})
	w.OnFrame(frame(t, "alpha", wire.ClassEvent, 4))
	if w.Failed() != 1 || w.Written() != 2 {
		t.Fatalf("written=%d failed=%d", w.Written(), w.Failed())
	}
}

// TestEventWriterSinkDrivenDemand: pb: rows on demand-gated classes whose
// mode demands reads join host:<inst>:<class> upstream — wildcard rows
// expand over the known instances (re-evaluated on hello/summary), exact
// auto rows win over a wildcard always row, always-classes never join, and
// dropping the row leaves the room.
func TestEventWriterSinkDrivenDemand(t *testing.T) {
	app := newPolicyApp(t)
	d := &recDemand{}
	var mu sync.Mutex
	known := []string{"alpha", "gamma"}
	w := NewEventWriter(app, EventWriterOptions{Demand: d, Logf: t.Logf, Instances: func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), known...)
	}})

	w.SetCapturePolicies([]capture.Policy{
		{Instance: "*", Class: wire.ClassTick, Mode: capture.ModeAlways, Sink: "pb:game_events"},
		{Instance: "alpha", Class: wire.ClassTick, Mode: capture.ModeAuto, Sink: "pb:game_events"},
		{Instance: "beta", Class: wire.ClassObjects, Mode: capture.ModeAlways, Sink: "pb:game_events"},
		{Instance: "*", Class: wire.ClassEvent, Mode: capture.ModeAlways, Sink: "pb:game_events"},
		{Instance: "*", Class: wire.ClassDebug, Mode: capture.ModeAlways, Sink: "file:/tmp/d.ndjson"},
	})
	want := []string{"join host:beta:objects", "join host:beta:tick", "join host:gamma:tick"}
	if got := d.take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v\nwant %v", got, want)
	}
	if got := w.Rooms(); !reflect.DeepEqual(got, []string{"host:beta:objects", "host:beta:tick", "host:gamma:tick"}) {
		t.Fatalf("rooms = %v", got)
	}

	// A new instance shows up (hello / summary): wildcard rows expand to it.
	mu.Lock()
	known = append(known, "delta")
	mu.Unlock()
	w.OnFrame(frame(t, "", wire.ClassSummary, 1))
	if got := d.take(); !reflect.DeepEqual(got, []string{"join host:delta:tick"}) {
		t.Fatalf("after summary: %v", got)
	}
	w.OnFrame(frame(t, "", wire.ClassHello, 1))
	if got := d.take(); len(got) != 0 {
		t.Fatalf("hello without change must be quiet: %v", got)
	}

	// Rows change: the wildcard always row is gone, beta keeps objects.
	w.SetCapturePolicies([]capture.Policy{
		{Instance: "beta", Class: wire.ClassObjects, Mode: capture.ModeAlways, Sink: "pb:game_events"},
	})
	want = []string{"leave host:beta:tick", "leave host:delta:tick", "leave host:gamma:tick"}
	if got := d.take(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after drop: %v\nwant %v", got, want)
	}
	w.SetCapturePolicies(nil)
	if got := d.take(); !reflect.DeepEqual(got, []string{"leave host:beta:objects"}) {
		t.Fatalf("after clear: %v", got)
	}
	if len(w.Rooms()) != 0 {
		t.Fatalf("rooms = %v", w.Rooms())
	}
}

// TestEventWriterFedByProvider: the split captureprovider feeds the writer
// through the Pusher interface — ReloadCapturePolicies + the PB hooks.
func TestEventWriterFedByProvider(t *testing.T) {
	app := newPolicyApp(t)
	col := ensurePolicyCollection(t, app)
	d := &recDemand{}
	w := NewEventWriter(app, EventWriterOptions{Demand: d, Logf: t.Logf})
	RegisterCapturePolicyHooks(app, w)
	if err := ReloadCapturePolicies(app, w); err != nil {
		t.Fatal(err)
	}
	if len(w.CapturePolicies()) != 0 {
		t.Fatalf("policies = %+v", w.CapturePolicies())
	}
	rec := savePolicy(t, app, col, capture.Policy{Instance: "alpha", Class: wire.ClassTick, Mode: capture.ModeAlways, Sink: "pb:game_events"})
	if got := w.CapturePolicies(); len(got) != 1 || got[0].Sink != "pb:game_events" {
		t.Fatalf("after create: %+v", got)
	}
	if got := d.take(); !reflect.DeepEqual(got, []string{"join host:alpha:tick"}) {
		t.Fatalf("demand after create: %v", got)
	}
	if err := app.Delete(rec); err != nil {
		t.Fatal(err)
	}
	if got := d.take(); !reflect.DeepEqual(got, []string{"leave host:alpha:tick"}) {
		t.Fatalf("demand after delete: %v", got)
	}
}
