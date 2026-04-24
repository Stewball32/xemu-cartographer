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
	"time"
)

// Watcher polls a directory for .sock files and invokes callbacks when sockets
// appear (and are connectable) or disappear.
type Watcher struct {
	dir      string
	interval time.Duration
	onAdd    func(name, sockPath string)
	onRemove func(name string)

	// known tracks sockets we have called onAdd for.
	known map[string]struct{}
	// failCount tracks consecutive dial failures for known sockets.
	failCount map[string]int
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
	}
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
		current[base] = filepath.Join(w.dir, name)
	}

	// Detect removed sockets.
	for name := range w.known {
		if _, ok := current[name]; !ok {
			delete(w.known, name)
			delete(w.failCount, name)
			log.Printf("discovery: socket removed: %s", name)
			w.onRemove(name)
		}
	}

	// Check known sockets for staleness (dial failure).
	for name := range w.known {
		path, ok := current[name]
		if !ok {
			continue // already handled above
		}
		if !dialSocket(path) {
			w.failCount[name]++
			if w.failCount[name] >= maxDialFails {
				delete(w.known, name)
				delete(w.failCount, name)
				log.Printf("discovery: socket stale (%d consecutive dial failures): %s", maxDialFails, name)
				w.onRemove(name)
			}
		} else {
			w.failCount[name] = 0
		}
	}

	// Detect new or re-appeared sockets.
	for name, path := range current {
		if _, ok := w.known[name]; ok {
			continue // already tracked
		}
		if !dialSocket(path) {
			continue // not connectable yet, try next poll
		}
		log.Printf("discovery: socket found: %s (%s)", name, path)
		w.known[name] = struct{}{}
		delete(w.failCount, name)
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
