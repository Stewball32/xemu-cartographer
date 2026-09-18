package leaguescraper

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/xemu-cartographer/xc-scraper/hosthealth"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/runner"
	scraperiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"
)

// Adapter is the wire-mode scraperiface.Service (DESIGN-STEP8 §8.5): the
// league server's view of an xc-scraper daemon reached over xcclient. Every
// read that the in-process WireAdapter answered from *runner.Manager is
// answered here from the stream mirror (List / InstanceState / Membership /
// JoinReplay*) or proxied to the daemon's HTTP surface (Inspect, EventsReply,
// ProbeReply, the map / readout / health / diagnostics / host sources) and
// every mutation is a control call (Start / Stop → attach / detach; the
// PlayControl / HostControl setters → the /host routes).
//
// The Adapter also serves the per-principal hello (hello.go) and satisfies
// the route-level PlayControl / HostControl / MapSource / ReadoutSource /
// HealthSource interfaces (asserted in adapter_test.go — the route
// packages are not imported here).
//
// Fail-fast: every proxied call inherits the Ctl gate — while the stream is
// down it returns the "not found" / zero answer immediately
// (xcclient.ErrUpstreamDown) instead of waiting on a daemon that is away.
type Adapter struct {
	client *xcclient.Client
	mirror *xcclient.Mirror
	ctl    *xcclient.Ctl
	hello  upstreamHello

	inspect *memo[runner.InspectState]
	maps    *memo[runner.MapList]
	rows    *memo[[]xcclient.InstanceRow]

	logf func(string, ...any)
}

// Compile-time proof that the Adapter is what main.go stores in
// guards.Services.Scraper / hands to authzpb.NewDeps in wire mode.
var _ scraperiface.Service = (*Adapter)(nil)

const (
	// inspectTTL is the per-name Inspect cache window (§8.5: 250 ms +
	// single-flight — overlay_pov polls inspect from several tabs).
	inspectTTL = 250 * time.Millisecond
	// mapsTTL is the AvailableMaps cache window (§9: 2 s).
	mapsTTL = 2 * time.Second
	// rowsTTL is the Rows (GET /api/instances) cache window (§6.1: the
	// admin list reads the daemon's read API through a 1 s cache).
	rowsTTL = time.Second
)

// NewAdapter builds the adapter over c's mirror and the control client ctl
// (normally xcclient.NewCtl(c, controlToken)).
func NewAdapter(c *xcclient.Client, ctl *xcclient.Ctl) *Adapter {
	a := &Adapter{
		client:  c,
		mirror:  c.Mirror(),
		ctl:     ctl,
		inspect: newMemo[runner.InspectState](inspectTTL),
		maps:    newMemo[runner.MapList](mapsTTL),
		rows:    newMemo[[]xcclient.InstanceRow](rowsTTL),
		logf:    log.Printf,
	}
	a.hello.reset()
	return a
}

// Client returns the stream client the adapter reads through.
func (a *Adapter) Client() *xcclient.Client { return a.client }

// Ctl returns the daemon control client.
func (a *Adapter) Ctl() *xcclient.Ctl { return a.ctl }

// OnFrame is the xcclient.Config.OnFrame hook the adapter needs: it records
// the upstream hello's protocol fields for HelloPayloadFor. Compose it with
// the rebroadcaster's OnFrame when wiring the client.
func (a *Adapter) OnFrame(f xcclient.Frame) {
	a.hello.observe(f)
}

// --- mirror-backed views (scraperiface.Inspect / State / Membership / JoinReplay)

// List returns the mirrored instances sorted by name.
func (a *Adapter) List() []runner.Info { return a.mirror.List() }

// InstanceState is the container-detail view from the mirror.
func (a *Adapter) InstanceState(name string) (runner.InstanceState, bool) {
	return a.mirror.InstanceState(name)
}

// Membership projects every mirrored instance's roster identities.
func (a *Adapter) Membership() []scraperiface.ContainerMembership {
	return a.mirror.Membership()
}

// JoinReplayMessages replays the cached per-class frames of every instance
// (raw wire.Message bytes, exactly as the daemon framed them).
func (a *Adapter) JoinReplayMessages() [][]byte { return a.mirror.JoinReplayMessages() }

// JoinReplayForInstance replays one instance's cached frames.
func (a *Adapter) JoinReplayForInstance(name string) [][]byte {
	return a.mirror.JoinReplayForInstance(name)
}

// JoinReplayForInstanceClass replays one (instance, class) frame.
func (a *Adapter) JoinReplayForInstanceClass(name, class string) [][]byte {
	return a.mirror.JoinReplayForInstanceClass(name, class)
}

// JoinReplayForHostAll replays the cached summary frame.
func (a *Adapter) JoinReplayForHostAll() [][]byte { return a.mirror.JoinReplayForHostAll() }

// --- read-API proxies

func (a *Adapter) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), xcclient.DefaultCtlTimeout)
}

// Inspect proxies GET /api/instances/{n}/inspect through a 250 ms
// single-flight cache: concurrent callers share one daemon round trip.
func (a *Adapter) Inspect(name string) (runner.InspectState, bool) {
	return a.inspect.get(name, func() (runner.InspectState, bool) {
		ctx, cancel := a.ctx()
		defer cancel()
		st, err := a.ctl.Inspect(ctx, name)
		if err != nil {
			a.note("inspect", name, err)
			return runner.InspectState{}, false
		}
		return st, true
	})
}

// Rows proxies GET /api/instances through a 1 s single-flight cache (§6.1,
// §16 pod list): every running instance's Info plus the rows the daemon's
// attach side is still retrying or holding — phase "attaching"/"detached",
// attach{kind,addr} and the last attach error — which the stream mirror
// (List) never sees. ok=false while the daemon is away or the read fails;
// the admin list route answers 503 then rather than an empty mirror.
func (a *Adapter) Rows() ([]xcclient.InstanceRow, bool) {
	return a.rows.get("", func() ([]xcclient.InstanceRow, bool) {
		ctx, cancel := a.ctx()
		defer cancel()
		rows, err := a.ctl.Instances(ctx)
		if err != nil {
			a.note("instances", "", err)
			return nil, false
		}
		if rows == nil {
			rows = []xcclient.InstanceRow{}
		}
		return rows, true
	})
}

// EventsReply proxies GET /api/instances/{n}/events; the framed bytes come
// back verbatim (room host:<n>).
func (a *Adapter) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	ctx, cancel := a.ctx()
	defer cancel()
	raw, err := a.ctl.Events(ctx, instance, sinceTick, types)
	if err != nil {
		a.note("events", instance, err)
		return nil, false
	}
	return raw, true
}

// ProbeReply proxies GET /api/instances/{n}/probe verbatim.
func (a *Adapter) ProbeReply(instance string) ([]byte, bool) {
	ctx, cancel := a.ctx()
	defer cancel()
	raw, err := a.ctl.Probe(ctx, instance)
	if err != nil {
		a.note("probe", instance, err)
		return nil, false
	}
	return raw, true
}

// AvailableMaps proxies GET /api/instances/{n}/maps (2 s cache); an
// unreachable daemon yields the empty list like an unread carousel.
func (a *Adapter) AvailableMaps(name string) runner.MapList {
	ml, _ := a.maps.get(name, func() (runner.MapList, bool) {
		ctx, cancel := a.ctx()
		defer cancel()
		ml, err := a.ctl.Maps(ctx, name)
		if err != nil {
			a.note("maps", name, err)
			return runner.MapList{}, false
		}
		return ml, true
	})
	return ml
}

// Readout proxies GET /api/instances/{n}/readout.
func (a *Adapter) Readout(name string) (hostrunner.ScraperReadout, bool) {
	ctx, cancel := a.ctx()
	defer cancel()
	ro, err := a.ctl.Readout(ctx, name)
	if err != nil {
		a.note("readout", name, err)
		return hostrunner.ScraperReadout{}, false
	}
	return ro, true
}

// HostHealth proxies GET /api/instances/{n}/health.
func (a *Adapter) HostHealth(name string) (hosthealth.Health, bool) {
	ctx, cancel := a.ctx()
	defer cancel()
	hh, err := a.ctl.HostHealth(ctx, name)
	if err != nil {
		a.note("health", name, err)
		return hosthealth.Health{}, false
	}
	return hh, true
}

// Diagnostics proxies GET /api/instances/{n}/diagnostics and returns its
// host_runner snapshot (routes/scraper HostControl); an absent daemon
// yields the empty snapshot the registry answers for an unknown instance.
func (a *Adapter) Diagnostics(instance string) hostrunner.Diagnostics {
	ctx, cancel := a.ctx()
	defer cancel()
	d, err := a.ctl.Diagnostics(ctx, instance)
	if err != nil {
		a.note("diagnostics", instance, err)
		return hostrunner.Diagnostics{Instance: instance}
	}
	return d.HostRunner
}

// Status proxies GET /api/instances/{n}/host (PlayControl / HostControl);
// 404 (unknown instance or hostrunner off) is the not-present status.
func (a *Adapter) Status(instance string) hostrunner.Status {
	ctx, cancel := a.ctx()
	defer cancel()
	st, err := a.ctl.HostStatus(ctx, instance)
	if err != nil {
		a.note("host", instance, err)
		return hostrunner.Status{Instance: instance}
	}
	return st
}

// --- control-API proxies

// Start maps scraperiface.Lifecycle.Start(name, sock) onto POST
// /api/ctl/attach {name, addr:"unix:"+sock}. 202 = nil (the attach is
// asynchronous — failure surfaces as the instance phase); 409
// already_running and 400 invalid_name map to the runner sentinels the
// scraper routes already switch on.
func (a *Adapter) Start(name, sock string) error {
	ctx, cancel := a.ctx()
	defer cancel()
	_, err := a.ctl.Attach(ctx, name, "unix:"+sock)
	if err == nil {
		a.rows.drop("") // the accepted attach is a new list row
	}
	return mapControlErr(err)
}

// Stop is DELETE /api/ctl/instances/{n}. A 404 (nothing attached) is a
// successful no-op like Manager.Stop on an unknown name.
func (a *Adapter) Stop(name string) error {
	ctx, cancel := a.ctx()
	defer cancel()
	err := a.ctl.Detach(ctx, name)
	if err == nil {
		a.rows.drop("") // the row's phase just changed
	}
	if errors.Is(err, &xcclient.Error{Status: http.StatusNotFound}) {
		return nil
	}
	return mapControlErr(err)
}

// SetAuthority is PlayControl / HostControl.SetAuthority over POST
// …/host {"authority"}; false when the daemon rejected or is away.
func (a *Adapter) SetAuthority(instance string, auth hostrunner.Authority) bool {
	ctx, cancel := a.ctx()
	defer cancel()
	_, err := a.ctl.SetAuthority(ctx, instance, auth.String())
	if err != nil {
		a.note("set authority", instance, err)
		return false
	}
	return true
}

// SetReady is PlayControl.SetReady over PUT …/host/ready.
func (a *Adapter) SetReady(instance string, ready bool) bool {
	ctx, cancel := a.ctx()
	defer cancel()
	if _, err := a.ctl.SetReady(ctx, instance, ready); err != nil {
		a.note("set ready", instance, err)
		return false
	}
	return true
}

// SetSelection is PlayControl.SetSelection. Only the two names travel
// (§6.2): the daemon owns the live carousel and computes the D-pad steps
// itself, so mapSteps / gametypeSteps are ignored here.
func (a *Adapter) SetSelection(instance, mapName string, mapSteps int, gametypeName string, gametypeSteps int) bool {
	_, _ = mapSteps, gametypeSteps
	ctx, cancel := a.ctx()
	defer cancel()
	if _, err := a.ctl.SetSelection(ctx, instance, mapName, gametypeName); err != nil {
		a.note("set selection", instance, err)
		return false
	}
	return true
}

// ClearSelection is PlayControl.ClearSelection over DELETE …/host/selection.
func (a *Adapter) ClearSelection(instance string) bool {
	ctx, cancel := a.ctx()
	defer cancel()
	if _, err := a.ctl.ClearSelection(ctx, instance); err != nil {
		a.note("clear selection", instance, err)
		return false
	}
	return true
}

// mapControlErr translates the daemon's attach / detach errors onto the
// sentinels routes/scraper switches on.
func mapControlErr(err error) error {
	var de *xcclient.Error
	if errors.As(err, &de) {
		switch de.Code {
		case "already_running":
			return fmt.Errorf("%w: %s", runner.ErrAlreadyRunning, de.Message)
		case "invalid_name", "bad_addr":
			return fmt.Errorf("%w: %s", runner.ErrInvalidName, de.Message)
		}
	}
	return err
}

// note logs a proxied-call failure, except the fail-fast case (the stream
// client already logs its reconnect attempts) and plain 404s (the routes
// answer those themselves).
func (a *Adapter) note(what, instance string, err error) {
	if errors.Is(err, xcclient.ErrUpstreamDown) || errors.Is(err, &xcclient.Error{Status: http.StatusNotFound}) {
		return
	}
	a.logf("leaguescraper[%s]: %s: %v", instance, what, err)
}

// --- XC_SCRAPER_URL validation (§11)

// ErrBadUpstreamURL is the boot error for an XC_SCRAPER_URL that is not a
// bare http(s)://host[:port].
var ErrBadUpstreamURL = errors.New("leaguescraper: XC_SCRAPER_URL must be http(s)://host[:port]")

// ValidateUpstreamURL accepts exactly http(s)://host[:port] (no path,
// query, fragment or userinfo — ws:// is derived by xcclient, never given).
func ValidateUpstreamURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w (%q: %v)", ErrBadUpstreamURL, raw, err)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https",
		u.Host == "" || u.User != nil,
		u.Path != "" && u.Path != "/",
		u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || u.Opaque != "":
		return fmt.Errorf("%w (got %q)", ErrBadUpstreamURL, raw)
	}
	return nil
}

// --- memo: per-name TTL cache with single-flight

type memoEntry[T any] struct {
	val  T
	ok   bool
	at   time.Time
	done chan struct{} // non-nil while a fetch is in flight
}

type memo[T any] struct {
	ttl     time.Duration
	now     func() time.Time
	mu      sync.Mutex
	entries map[string]*memoEntry[T]
}

func newMemo[T any](ttl time.Duration) *memo[T] {
	return &memo[T]{ttl: ttl, now: time.Now, entries: make(map[string]*memoEntry[T])}
}

// get answers from a fresh entry, joins an in-flight fetch, or runs fetch
// itself and publishes the result to every waiter.
func (m *memo[T]) get(name string, fetch func() (T, bool)) (T, bool) {
	m.mu.Lock()
	if e := m.entries[name]; e != nil {
		if e.done != nil {
			done := e.done
			m.mu.Unlock()
			<-done
			m.mu.Lock()
			val, ok := e.val, e.ok
			m.mu.Unlock()
			return val, ok
		}
		if m.now().Sub(e.at) < m.ttl {
			val, ok := e.val, e.ok
			m.mu.Unlock()
			return val, ok
		}
	}
	e := &memoEntry[T]{done: make(chan struct{})}
	m.entries[name] = e
	m.prune()
	m.mu.Unlock()

	val, ok := fetch()

	m.mu.Lock()
	e.val, e.ok, e.at = val, ok, m.now()
	done := e.done
	e.done = nil
	m.mu.Unlock()
	close(done)
	return val, ok
}

// drop forgets name's settled entry so the next get refetches (an
// in-flight fetch is left to publish; it is already newer than the caller).
func (m *memo[T]) drop(name string) {
	m.mu.Lock()
	if e := m.entries[name]; e != nil && e.done == nil {
		delete(m.entries, name)
	}
	m.mu.Unlock()
}

// prune drops settled entries older than a minute (called under mu).
func (m *memo[T]) prune() {
	if len(m.entries) < 64 {
		return
	}
	cutoff := m.now().Add(-time.Minute)
	for k, e := range m.entries {
		if e.done == nil && e.at.Before(cutoff) {
			delete(m.entries, k)
		}
	}
}
