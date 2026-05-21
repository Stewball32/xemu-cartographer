package sinks

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// PBSink persists each envelope as one record in a PocketBase collection.
// The destination is identified by collection name (the "rest" after the
// "pb:" scheme prefix); the collection must already exist when Write is
// called or every write fails with a logged error from app.Save.
//
// The record carries the envelope's flat fields (instance, type, seq,
// tick, ts) as separate columns so they're indexable, plus the inner
// payload as `data` for queryable JSON access. The PR 18 schema for
// game_events is in internal/pocketbase/schema/game_events.go; future
// destinations (game_history etc.) follow the same shape.
//
// Concurrency: collection lookup is cached after the first successful
// hit; Write is safe to call from the runner loop without external
// serialisation.
type PBSink struct {
	app        core.App
	collection string

	mu     sync.Mutex
	cached *core.Collection
}

// NewPBSink returns a sink that creates one record per envelope in the
// named PocketBase collection. The collection isn't validated at
// construction time — a typo surfaces on the first Write — so applying
// a policy with pb:<bad> doesn't block the rest of the runner.
func NewPBSink(app core.App, collection string) *PBSink {
	return &PBSink{app: app, collection: collection}
}

// Collection returns the destination collection name. Used by the
// runner's sinkManager for spec-change detection without exposing
// internals.
func (s *PBSink) Collection() string { return s.collection }

func (s *PBSink) Write(envBytes []byte) error {
	if s.app == nil {
		return fmt.Errorf("pb sink: nil app")
	}
	var env scraper.Envelope
	if err := json.Unmarshal(envBytes, &env); err != nil {
		return fmt.Errorf("pb sink: parse envelope: %w", err)
	}

	col, err := s.resolveCollection()
	if err != nil {
		return err
	}

	rec := core.NewRecord(col)
	rec.Set("instance", env.Instance)
	rec.Set("type", env.Type)
	rec.Set("seq", env.Seq)
	rec.Set("tick", env.Tick)
	rec.Set("ts", env.Ts)
	rec.Set("data", env.Data)
	return s.app.Save(rec)
}

// Close is a no-op — there's no per-sink resource to release; PB record
// writes go through the shared app handle that the server owns.
func (s *PBSink) Close() error { return nil }

func (s *PBSink) resolveCollection() (*core.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil {
		return s.cached, nil
	}
	col, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("pb sink: collection %q: %w", s.collection, err)
	}
	s.cached = col
	return col, nil
}

// RegisterPBSink wires the pb: scheme into the package-level registry.
// Call once from main.go after PocketBase comes up but before
// scraper.Manager.ReloadCapturePolicies — otherwise a policy with a
// pb: spec loaded at startup would fail with "unknown scheme".
func RegisterPBSink(app core.App) {
	Register("pb", func(rest string, _ map[string]string) (Sink, error) {
		if rest == "" {
			return nil, fmt.Errorf("pb sink: missing collection name in spec")
		}
		return NewPBSink(app, rest), nil
	})
}
