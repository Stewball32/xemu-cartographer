// Package discovery watches a directory for QMP Unix sockets and notifies
// callers when instances appear or disappear.
package discovery

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watcher polls a directory for .sock files and invokes callbacks when sockets
// appear (and are connectable) or disappear.
type Watcher struct {
	dir      string
	interval time.Duration
	onAdd    func(name, sockPath string)
	onRemove func(name string)

	mu sync.Mutex
	// known tracks sockets we have called onAdd for.
	known map[string]struct{}
	// failCount tracks consecutive dial failures for known sockets.
	failCount map[string]int
	// everSeen tracks names we have ever logged "socket found" for, so retries
	// after Forget don't re-log the same line every poll.
	everSeen map[string]struct{}
}

const maxDialFails = 3

// NewWatcher creates a Watcher that polls dir every interval.
// onAdd is called when a new connectable .sock file is found (name is filename
// minus extension). onRemove is called when a previously-known socket disappears
// or becomes unreachable for maxDialFails consecutive polls.
func NewWatcher(dir string, interval time.Duration, onAdd func(name, sockPath string), onRemove func(name string)) *Watcher {
	return &Watcher{
		dir:       dir,
		interval:  interval,
		onAdd:     onAdd,
		onRemove:  onRemove,
		known:     make(map[string]struct{}),
		failCount: make(map[string]int),
		everSeen:  make(map[string]struct{}),
	}
}

// Forget drops name from the known set so the next poll re-emits onAdd.
// Use this from the onAdd callback when an attach attempt failed transiently
// (e.g. xemu still booting and guest VAs not yet mapped, dashboard with no
// game loaded). The "socket found" log line is suppressed on retry; the
// caller is responsible for not spamming its own error logs.
func (w *Watcher) Forget(name string) {
	w.mu.Lock()
	delete(w.known, name)
	delete(w.failCount, name)
	w.mu.Unlock()
}

// Run polls the directory until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	// Do an immediate first poll before entering the ticker loop.
	w.poll()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll()
		}
	}
}

func (w *Watcher) poll() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		log.Printf("discovery: readdir %s: %v", w.dir, err)
		return
	}

	// Build set of .sock files currently in the directory.
	current := make(map[string]string) // name → full path
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sock") {
			continue
		}
		base := strings.TrimSuffix(name, ".sock")
		// "all" is reserved for the host:all aggregate room (see
		// internal/websocket/rooms/host.go). The scraper Manager.Start
		// chokepoint rejects it, but skipping it here too avoids spamming
		// the log with a failed-start error every poll if someone names a
		// socket "all.sock".
		if base == "all" {
			log.Printf("discovery: skipping reserved socket name %q — rename to anything else", name)
			continue
		}
		current[base] = filepath.Join(w.dir, name)
	}

	// Snapshot the known set so we can iterate without holding the lock during
	// dialSocket (500ms timeouts) or the user-supplied onAdd/onRemove callbacks
	// (which may call Forget). Mutations happen under w.mu inside the loops.
	w.mu.Lock()
	currentNames := make([]string, 0, len(w.known))
	for name := range w.known {
		currentNames = append(currentNames, name)
	}
	w.mu.Unlock()

	// Detect removed sockets (in known but no longer on disk).
	for _, name := range currentNames {
		if _, ok := current[name]; !ok {
			w.mu.Lock()
			delete(w.known, name)
			delete(w.failCount, name)
			delete(w.everSeen, name)
			w.mu.Unlock()
			log.Printf("discovery: socket removed: %s", name)
			w.onRemove(name)
		}
	}

	// Check known sockets for staleness (dial failure).
	for _, name := range currentNames {
		path, ok := current[name]
		if !ok {
			continue // already handled above
		}
		if dialSocket(path) {
			w.mu.Lock()
			w.failCount[name] = 0
			w.mu.Unlock()
			continue
		}
		w.mu.Lock()
		w.failCount[name]++
		stale := w.failCount[name] >= maxDialFails
		if stale {
			delete(w.known, name)
			delete(w.failCount, name)
			delete(w.everSeen, name)
		}
		w.mu.Unlock()
		if stale {
			log.Printf("discovery: socket stale (%d consecutive dial failures): %s", maxDialFails, name)
			w.onRemove(name)
		}
	}

	// Detect new or re-appeared sockets (post-Forget).
	for name, path := range current {
		w.mu.Lock()
		_, alreadyKnown := w.known[name]
		w.mu.Unlock()
		if alreadyKnown {
			continue
		}
		if !dialSocket(path) {
			continue // not connectable yet, try next poll
		}
		w.mu.Lock()
		// Re-check under the lock to handle a Forget/poll race.
		if _, alreadyKnown := w.known[name]; alreadyKnown {
			w.mu.Unlock()
			continue
		}
		_, seenBefore := w.everSeen[name]
		w.known[name] = struct{}{}
		w.everSeen[name] = struct{}{}
		delete(w.failCount, name)
		w.mu.Unlock()

		if !seenBefore {
			log.Printf("discovery: socket found: %s (%s)", name, path)
		}
		w.onAdd(name, path)
	}
}

// dialSocket attempts a quick TCP-style dial on a Unix socket to verify the
// remote end is listening.
func dialSocket(path string) bool {
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
