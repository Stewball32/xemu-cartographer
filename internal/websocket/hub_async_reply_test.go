package websocket

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/authztest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
	scraperiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/scraper"
)

// stallingScraper answers EventsReply / ProbeReply only after release is
// closed, and counts the calls.
type stallingScraper struct {
	scraperiface.Service
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func newStallingScraper() *stallingScraper {
	return &stallingScraper{entered: make(chan struct{}, 8), release: make(chan struct{})}
}

func (s *stallingScraper) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	s.calls.Add(1)
	s.entered <- struct{}{}
	<-s.release
	return []byte("events " + instance), true
}

func (s *stallingScraper) ProbeReply(instance string) ([]byte, bool) {
	s.calls.Add(1)
	s.entered <- struct{}{}
	<-s.release
	return []byte("probe " + instance), true
}

// TestRequestReplyAfterDisconnect (§13 "reply after disconnect"): the
// dispatcher hands request_events / request_probe to a goroutine and moves
// on; when the Replayer finally answers after the Hub has removed the
// sender, the frame is discarded — send is never closed, so there is
// nothing to panic on — while a sender still connected gets its reply.
func TestRequestReplyAfterDisconnect(t *testing.T) {
	for _, msgType := range []string{"request_events", "request_probe"} {
		t.Run(msgType, func(t *testing.T) {
			h := NewHub(nil)
			t.Cleanup(h.Stop)
			stub := newStallingScraper()
			h.SetServices(&guards.Services{Scraper: stub, WS: h})
			h.deps = &authztest.FakeDeps{}
			gone := addTestClient(h, machineKey("k1"), "host:pod-a")
			stays := addTestClient(h, machineKey("k2"), "host:pod-a")

			returned := make(chan struct{})
			go func() {
				h.dispatch(incomingMsg{msg: Message{Type: msgType}, sender: gone})
				h.dispatch(incomingMsg{msg: Message{Type: msgType}, sender: stays})
				close(returned)
			}()
			select {
			case <-returned:
			case <-time.After(5 * time.Second):
				t.Fatal("dispatch blocked on the stalled Replayer")
			}
			for i := 0; i < 2; i++ {
				select {
				case <-stub.entered:
				case <-time.After(5 * time.Second):
					t.Fatal("Replayer was not asked twice")
				}
			}

			h.removeClient(gone)
			if !gone.closed() || connected(h, gone) {
				t.Fatal("removeClient did not close the sender")
			}
			close(stub.release)

			waitFor(t, "the surviving sender's reply", func() bool { return len(stays.send) == 1 })
			time.Sleep(20 * time.Millisecond)
			if got := drainSend(gone); len(got) != 0 {
				t.Fatalf("removed sender was handed %q", got)
			}
			if got := drainSend(stays); len(got) != 1 || got[0] != msgType[len("request_"):]+" pod-a" {
				t.Fatalf("surviving sender got %q", got)
			}
			if n := stub.calls.Load(); n != 2 {
				t.Fatalf("Replayer asked %d times, want 2", n)
			}
		})
	}
}
