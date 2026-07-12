package main

import (
	"os"
	"strconv"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
	"github.com/Stewball32/xemu-cartographer/internal/reaper"
	scrapermgr "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
)

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
