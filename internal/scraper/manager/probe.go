package manager

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// envelopeTypeProbe is the wire type for a request_probe response.
// On-demand only — never broadcast. Probe is the developer scratch
// space for finding/debugging memory values; running BuildScoreProbe
// every tick (its old home, the per-tick debug envelope) was wasted
// memory-read work whenever the probe page wasn't open.
const envelopeTypeProbe = "probe"

// ProbePayload is the data for a probe-class envelope. Mirrors the
// shape `internal/scraper.GameReader` returns: state_inputs are the
// raw values the plugin's ReadGameState sampled, score_probe is the
// free-form bag of every candidate address the plugin knows about.
type ProbePayload struct {
	StateInputs scraper.StateInputs `json:"state_inputs"`
	ScoreProbe  scraper.ScoreProbe  `json:"score_probe"`
}

// probeRequest carries a one-shot reply channel. The loop goroutine
// fills the payload (by calling reader.LastStateInputs +
// reader.BuildScoreProbe) and sends back exactly one value, then
// closes — so a reader on the other side can range or single-recv
// without leaking goroutines.
type probeRequest struct {
	reply chan ProbePayload
}

// probeReplyTimeout bounds how long ProbeReply waits on the loop. One
// tick is ~33ms (30Hz Live); allow a generous multiple to absorb
// load spikes and BuildScoreProbe's six memory-read passes.
const probeReplyTimeout = 2 * time.Second

// ProbeReply runs the on-demand probe readers via the runner's loop
// goroutine and returns the marshaled envelope bytes addressed to
// host:<instance>. Returns (nil, false) when the instance has no
// runner attached, the runner is in Idle (no reader bound), or the
// loop didn't service the request before probeReplyTimeout.
//
// Lives on the manager so request_probe handlers can call across the
// import-cycle-free `scraperiface` boundary without depending on
// runner internals.
func (m *Manager) ProbeReply(instance string) ([]byte, bool) {
	m.mu.Lock()
	r, ok := m.runners[instance]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}

	// Snapshot phase + tick under cacheMu so the reply envelope's
	// metadata reflects the runner's current state regardless of what
	// the loop does between here and request servicing.
	c := r.readCache()

	var payload ProbePayload
	if c.Phase == PhaseIdle {
		// No reader bound — return an empty probe so the UI shows
		// "no data" rather than hanging on the timeout.
		payload = ProbePayload{
			StateInputs: scraper.StateInputs{},
			ScoreProbe:  scraper.ScoreProbe{},
		}
	} else {
		reply := make(chan ProbePayload, 1)
		select {
		case r.probeReqCh <- probeRequest{reply: reply}:
		default:
			// Queue full — too many concurrent probe requests. The WS
			// handler treats this as a transient failure; the next
			// request after the loop drains will succeed.
			return nil, false
		}
		select {
		case payload = <-reply:
		case <-time.After(probeReplyTimeout):
			return nil, false
		}
	}

	env := scraper.MakeEnvelope(envelopeTypeProbe, instance, r.nextSeq(envelopeTypeProbe), c.EngineTick, payload)
	msgBytes, ok := marshalRoomMessage(instance, r.hostRoom, env)
	if !ok {
		return nil, false
	}
	return msgBytes, true
}

// drainProbeRequests services every probe request queued since the
// last drain. Loop-goroutine-only — the reader's internal caches and
// the inst.Mem reads underneath BuildScoreProbe are exclusively the
// loop's responsibility. Called once per tick in the Ready / Live
// phase loops after the reader's per-tick work is complete.
//
// LastStateInputs is the map cached during the most recent
// ReadGameState (cheap pointer-return); BuildScoreProbe walks every
// candidate score / gametype / per-player offset (expensive — that's
// why it doesn't run per-tick anymore). safeMapAny strips
// NaN/un-marshalable values so one bad nested probe doesn't fail the
// whole reply (same rationale as the old per-tick debug envelope had).
func (r *runner) drainProbeRequests() {
	for {
		select {
		case req := <-r.probeReqCh:
			payload := ProbePayload{
				StateInputs: safeMapAny(r.reader.LastStateInputs()),
				ScoreProbe:  safeMapAny(r.reader.BuildScoreProbe()),
			}
			req.reply <- payload
			close(req.reply)
		default:
			return
		}
	}
}
