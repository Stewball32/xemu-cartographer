package handlers

import (
	"sync"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// slowScraper is a Replayer whose replies block until release is closed —
// the wire-mode adapter waiting on an upstream HTTP hop. entered is closed
// the first time a reply is asked for.
type slowScraper struct {
	scraperiface.Service
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func newSlowScraper() *slowScraper {
	return &slowScraper{entered: make(chan struct{}), release: make(chan struct{})}
}

func (s *slowScraper) wait(instance string) {
	s.once.Do(func() { close(s.entered) })
	<-s.release
}

func (s *slowScraper) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	s.wait(instance)
	return []byte("events " + instance), true
}

func (s *slowScraper) ProbeReply(instance string) ([]byte, bool) {
	s.wait(instance)
	return []byte("probe " + instance), true
}

// replySink is a goroutine-safe SendRaw recorder.
type replySink struct {
	mu   sync.Mutex
	sent []string
}

func (r *replySink) send(data []byte) {
	r.mu.Lock()
	r.sent = append(r.sent, string(data))
	r.mu.Unlock()
}

func (r *replySink) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sent...)
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestRequestHandlersAsync_SlowReplayerNeverBlocksDispatch (§13): the
// registered request_events / request_probe handlers return to the
// dispatcher immediately even when the Replayer stalls, and the reply
// still reaches SendRaw once the upstream answers.
func TestRequestHandlersAsync_SlowReplayerNeverBlocksDispatch(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		payload string
		rooms   []string
		want    string
	}{
		{"request_events via rooms", "request_events", "", []string{"host:pod-a:event"}, "events pod-a"},
		{"request_probe explicit instance", "request_probe", `{"instance":"pod-b"}`, nil, "probe pod-b"},
		{"request_probe via rooms", "request_probe", "", []string{"host:pod-a"}, "probe pod-a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler, ok := Get(tc.msgType)
			if !ok {
				t.Fatalf("%s has no handler registered", tc.msgType)
			}
			stub := newSlowScraper()
			sink := &replySink{}
			e := &Event{
				Services:  &guards.Services{Scraper: stub},
				Authz:     &authztest.FakeDeps{},
				Principal: machine("scraper-key", "scraper.*"),
				Type:      tc.msgType,
				SendRaw:   sink.send,
			}
			if tc.payload != "" {
				e.Payload = []byte(tc.payload)
			}
			if tc.rooms != nil {
				e.Rooms = func() []string { return tc.rooms }
			}

			returned := make(chan struct{})
			go func() {
				handler(e)
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(5 * time.Second):
				t.Fatal("handler did not return while the Replayer was stalled")
			}

			select {
			case <-stub.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("Replayer was never asked")
			}
			if got := sink.snapshot(); len(got) != 0 {
				t.Fatalf("reply %q sent before the Replayer answered", got)
			}

			close(stub.release)
			waitUntil(t, "the reply", func() bool { return len(sink.snapshot()) == 1 })
			if got := sink.snapshot(); got[0] != tc.want {
				t.Fatalf("reply = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRequestHandlersAsync_DeniedStillSilent: the goroutine hop does not
// loosen the per-instance decision — a principal without the scope gets no
// reply and the Replayer is never asked, exactly as on the synchronous path.
func TestRequestHandlersAsync_DeniedStillSilent(t *testing.T) {
	for _, msgType := range []string{"request_events", "request_probe"} {
		t.Run(msgType, func(t *testing.T) {
			handler, _ := Get(msgType)
			stub := newSlowScraper()
			close(stub.release)
			sink := &replySink{}
			handler(&Event{
				Services:  &guards.Services{Scraper: stub},
				Authz:     &authztest.FakeDeps{},
				Principal: machine("k1", "room.join:*"),
				Type:      msgType,
				Rooms:     func() []string { return []string{"host:pod-a"} },
				SendRaw:   sink.send,
			})
			time.Sleep(30 * time.Millisecond)
			select {
			case <-stub.entered:
				t.Fatal("Replayer asked for a denied principal")
			default:
			}
			if got := sink.snapshot(); len(got) != 0 {
				t.Fatalf("denied principal got %q", got)
			}
		})
	}
}
