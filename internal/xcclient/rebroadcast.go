package xcclient

import (
	"encoding/json"
	"log"
	"sync/atomic"

	"github.com/xemu-cartographer/xc-scraper/wire"
)

const (
	// HostRunnerType is the outer message type of host-runner events on both
	// sides: the daemon emits it on xc:hostrunner (X10), the league re-frames
	// it onto AdminRoom (today's studio shape, cmd/server/hostrunner.go).
	HostRunnerType = "host_runner"
	// AdminRoom is the league room host_runner events are delivered to.
	AdminRoom = "admin"
)

// rebroadcastClasses is the set of envelope classes forwarded downstream:
// wire.AllClasses() (which already excludes the reply-only hello / events /
// probe kinds — excluded again here as defence in depth, §8.4).
var rebroadcastClasses = func() map[string]bool {
	set := make(map[string]bool)
	for _, class := range wire.AllClasses() {
		set[class] = true
	}
	delete(set, wire.ClassHello)
	delete(set, wire.ClassEvents)
	delete(set, wire.ClassProbe)
	return set
}()

// Rebroadcaster forwards upstream frames onto the league hub (§8.4): every
// type:"scraper" frame whose envelope class is a room class and whose room
// is set goes to Hub.SendToRoomRaw(room, raw) byte-for-byte; type:
// "host_runner" frames are re-framed as {type:"host_runner", room:"admin",
// payload} on the admin room. Everything else (hello, events / probe
// replies, unknown types) is dropped. Use OnFrame as Config.OnFrame, or call
// it from a composed handler.
type Rebroadcaster struct {
	hub  HubPort
	logf func(string, ...any)

	forwarded atomic.Uint64
	dropped   atomic.Uint64
}

// NewRebroadcaster returns a Rebroadcaster over hub (nil ⇒ every frame is
// dropped) logging through logf (nil ⇒ log.Printf).
func NewRebroadcaster(hub HubPort, logf func(string, ...any)) *Rebroadcaster {
	if logf == nil {
		logf = log.Printf
	}
	return &Rebroadcaster{hub: hub, logf: logf}
}

// OnFrame forwards one upstream frame (Config.OnFrame signature).
func (r *Rebroadcaster) OnFrame(f Frame) {
	if r.hub == nil {
		r.dropped.Add(1)
		return
	}
	switch f.Msg.Type {
	case wire.TypeScraper:
		if f.Env == nil || f.Msg.Room == "" || !rebroadcastClasses[f.Env.Type] {
			r.dropped.Add(1)
			return
		}
		r.hub.SendToRoomRaw(f.Msg.Room, f.Raw)
		r.forwarded.Add(1)
	case HostRunnerType:
		data, err := json.Marshal(wire.Message{Type: HostRunnerType, Room: AdminRoom, Payload: f.Msg.Payload})
		if err != nil {
			r.logf("xcclient: marshal host_runner frame: %v", err)
			r.dropped.Add(1)
			return
		}
		r.hub.SendToRoomRaw(AdminRoom, data)
		r.forwarded.Add(1)
	default:
		r.dropped.Add(1)
	}
}

// Forwarded is the number of frames handed to the hub so far.
func (r *Rebroadcaster) Forwarded() uint64 { return r.forwarded.Load() }

// Dropped is the number of frames filtered out so far.
func (r *Rebroadcaster) Dropped() uint64 { return r.dropped.Load() }
