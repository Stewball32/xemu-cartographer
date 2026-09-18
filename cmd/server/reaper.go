package main

import (
	"os"
	"strconv"
	"time"

	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	scrapermgr "github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/podman"
	"github.com/xemu-cartographer/xemu-cartographer/internal/reaper"
)

// reaperSource picks the reaper's activity source for the feed's mode: the
// embedded manager + host-runner registry in-process, the mirror + daemon
// /host route in wire mode (DESIGN-STEP8 §9). hostReg is nil in wire mode.
func (f *scraperFeed) reaperSource(hostReg *hostrunner.Registry) reaper.Source {
	if f.wire != nil {
		src := mirrorReaperSource{mirror: f.wire.Client.Mirror()}
		if f.hostrunner {
			src.host = f.wire.Adapter.Status
		}
		return src
	}
	return reaperSource{scr: f.mgr, host: hostReg}
}

// reaperRemover picks the remover: stop-then-remove in-process, podman
// remove alone in wire mode (the daemon detaches when the socket vanishes).
func (f *scraperFeed) reaperRemover(pod *podman.Manager) reaper.Remover {
	if f.wire != nil {
		return podmanReaperRemover{pod: pod}
	}
	return reaperRemover{scr: f.mgr, pod: pod}
}

// mirrorView is the slice of *xcclient.Mirror the wire-mode source reads.
type mirrorView interface {
	Instances() []string
	Summary() (wire.SummaryPayload, bool)
	Game(name string) *wire.GamePayload
}

// mirrorReaperSource implements reaper.Source over the wire-mode mirror:
// the instance set and phase come from the mirrored summary / game frames,
// the guest-machine count from the game roster, and the host-runner's
// machine count through the daemon's GET …/host (one call per instance per
// poll; host is nil when HOSTRUNNER_ENABLED is off and the Adapter answers
// zero values while the upstream is down, which reads as "not present").
type mirrorReaperSource struct {
	mirror mirrorView
	host   func(name string) hostrunner.Status
}

func (s mirrorReaperSource) Snapshot() []reaper.Snapshot {
	names := s.mirror.Instances()
	out := make([]reaper.Snapshot, 0, len(names))
	for _, name := range names {
		out = append(out, reaper.Snapshot{Instance: name, Active: s.active(name)})
	}
	return out
}

func (s mirrorReaperSource) active(name string) bool {
	if s.phase(name) == wire.PhaseLive {
		return true
	}
	if g := s.mirror.Game(name); g != nil && len(g.Machines) >= 2 {
		return true
	}
	if s.host != nil {
		if st := s.host(name); st.Present && st.MachineCount >= 2 {
			return true
		}
	}
	return false
}

// phase prefers the summary row (the aggregate the daemon keeps fresh even
// when no per-instance game frame has been demanded), then the game frame.
func (s mirrorReaperSource) phase(name string) wire.Phase {
	if sum, ok := s.mirror.Summary(); ok {
		for _, h := range sum.Hosts {
			if h.Instance == name {
				return h.Phase
			}
		}
	}
	if g := s.mirror.Game(name); g != nil {
		return g.Phase
	}
	return ""
}

// podmanReaperRemover is the wire-mode reaper.Remover: remove the container
// only — the daemon's --watch-dir detaches the runner when its QMP socket
// disappears, so there is nothing to stop league-side.
type podmanReaperRemover struct{ pod *podman.Manager }

func (r podmanReaperRemover) Reap(instance string) error { return r.pod.Remove(instance) }

// reaperSource implements reaper.Source over the live scraper + host-runner
// state. A box counts as ACTIVE (idle clock reset) when a match is live on it,
// or when a second System Link machine has joined the host — i.e. it's being
// played or people have gathered. Everything else is an empty lobby.
type reaperSource struct {
	scr  *scrapermgr.Manager
	host *hostrunner.Registry
}

func (s reaperSource) Snapshot() []reaper.Snapshot {
	infos := s.scr.List()
	out := make([]reaper.Snapshot, 0, len(infos))
	for _, info := range infos {
		out = append(out, reaper.Snapshot{Instance: info.Name, Active: s.active(info.Name)})
	}
	return out
}

// active decides whether an instance is being played / joined. Reads two cheap
// signals: the scraper's lifecycle phase ("live" = a match is running) and the
// host-runner's machine count (>= 2 = a guest console joined the host in the
// lobby). The phase signal works even without the host-runner; the machine
// count catches a pre-match lobby people are gathering in.
func (s reaperSource) active(name string) bool {
	if st, ok := s.scr.Inspect(name); ok {
		if st.Phase == "live" {
			return true
		}
		// Roster fallback (host-runner-independent): 2+ connected machines in
		// the scraped game data means a guest is present.
		if st.GameData != nil && len(st.GameData.Machines) >= 2 {
			return true
		}
	}
	if s.host != nil {
		if st := s.host.Status(name); st.Present && st.MachineCount >= 2 {
			return true
		}
	}
	return false
}

// reaperRemover implements reaper.Remover: stop the scraper runner, then remove
// the container (symmetric with the discovery watcher's onRemove path).
type reaperRemover struct {
	scr *scrapermgr.Manager
	pod *podman.Manager
}

func (r reaperRemover) Reap(instance string) error {
	// Stop the runner up-front so its tick loop doesn't read a vanishing box.
	// Ignore the error — Stop is idempotent, and removing the container drops
	// the QMP socket, which the discovery watcher also auto-stops on.
	_ = r.scr.Stop(instance)
	return r.pod.Remove(instance)
}

// reaperIdleReporter adapts *reaper.Reaper to playroutes.IdleReporter so
// GET /api/play/current can surface the host's own reap countdown without the
// play package importing internal/reaper.
type reaperIdleReporter struct{ r *reaper.Reaper }

func (a reaperIdleReporter) IdleInfo(instance string) (idleSince, reapAt time.Time, warning, ok bool) {
	info := a.r.Info(instance)
	if !info.Idle {
		return time.Time{}, time.Time{}, false, false
	}
	return info.IdleSince, info.ReapAt, info.Warning, true
}

// reaperConfigFromEnv reads the idle-out reaper's tunables. REAPER_ENABLED
// gates it (default off); a disabled reaper returns a zero Config (zero
// IdleTimeout), which reaper.New treats as a no-op. Admin-configurable via the
// project's standard .env surface, consistent with CONTAINERS_*/HOSTRUNNER_*.
func reaperConfigFromEnv() reaper.Config {
	if !envBool("REAPER_ENABLED", false) {
		return reaper.Config{}
	}
	return reaper.Config{
		IdleTimeout: time.Duration(envInt("REAPER_IDLE_MINUTES", 15)) * time.Minute,
		WarnBefore:  time.Duration(envInt("REAPER_WARN_MINUTES", 2)) * time.Minute,
		Poll:        time.Duration(envInt("REAPER_POLL_SECONDS", 30)) * time.Second,
		NamePrefix:  envStr("REAPER_NAME_PREFIX", "play-"),
	}
}

// envInt reads an integer env var, falling back to def when unset/unparseable
// or non-positive.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// envStr reads a string env var, falling back to def when unset.
func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
