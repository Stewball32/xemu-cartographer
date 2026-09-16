package leaguescraper

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	playroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/play"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// Compile-time proof that the Adapter is every route-level source /
// control main.go injects in wire mode (DESIGN-STEP8 §8.5). The route
// packages are imported here, not in adapter.go, so leaguescraper never
// depends on them.
var (
	_ playroutes.PlayControl      = (*Adapter)(nil)
	_ playroutes.MapSource        = (*Adapter)(nil)
	_ scraperroutes.HostControl   = (*Adapter)(nil)
	_ scraperroutes.MapSource     = (*Adapter)(nil)
	_ scraperroutes.ReadoutSource = (*Adapter)(nil)
	_ scraperroutes.HealthSource  = (*Adapter)(nil)
)

// TestAdapterFailFastWhenDown: with the stream down every proxied call
// answers its "absent" value immediately, no HTTP request is made, and
// the mirror-backed views still serve.
func TestAdapterFailFastWhenDown(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, func() bool { return false })
	storeFixture(t, a, "game")

	start := time.Now()
	if err := a.Start("x", "/tmp/x.sock"); !errors.Is(err, xcclient.ErrUpstreamDown) {
		t.Fatalf("Start err = %v", err)
	}
	if err := a.Stop("x"); !errors.Is(err, xcclient.ErrUpstreamDown) {
		t.Fatalf("Stop err = %v", err)
	}
	if _, ok := a.Inspect("smoke1"); ok {
		t.Fatal("Inspect ok while down")
	}
	if _, ok := a.EventsReply("smoke1", 0, nil); ok {
		t.Fatal("EventsReply ok while down")
	}
	if _, ok := a.ProbeReply("smoke1"); ok {
		t.Fatal("ProbeReply ok while down")
	}
	if ml := a.AvailableMaps("smoke1"); ml.Available || len(ml.Maps) != 0 {
		t.Fatalf("AvailableMaps = %+v", ml)
	}
	if _, ok := a.Readout("smoke1"); ok {
		t.Fatal("Readout ok while down")
	}
	if _, ok := a.HostHealth("smoke1"); ok {
		t.Fatal("HostHealth ok while down")
	}
	if st := a.Status("smoke1"); st.Present || st.Instance != "smoke1" {
		t.Fatalf("Status = %+v", st)
	}
	if dg := a.Diagnostics("smoke1"); dg.Present || dg.Instance != "smoke1" {
		t.Fatalf("Diagnostics = %+v", dg)
	}
	if a.SetAuthority("smoke1", hostrunner.AuthAdmin) || a.SetReady("smoke1", true) ||
		a.SetSelection("smoke1", "m", 1, "g", 2) || a.ClearSelection("smoke1") {
		t.Fatal("a control setter reported success while down")
	}
	if el := time.Since(start); el > 50*time.Millisecond {
		t.Fatalf("fail-fast round took %v", el)
	}
	if n := d.hits.Load(); n != 0 {
		t.Fatalf("daemon saw %d request(s) while down", n)
	}
	if infos := a.List(); len(infos) != 1 || infos[0].Name != "smoke1" {
		t.Fatalf("List while down = %+v, want the mirrored instance", infos)
	}
}

// TestAdapterStartStopSentinels: Start posts {name, addr:"unix:"+sock};
// 202 → nil, 409 already_running → runner.ErrAlreadyRunning, 400
// invalid_name → runner.ErrInvalidName; Stop's 404 is a no-op.
func TestAdapterStartStopSentinels(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, nil)

	d.answer("POST", "/api/ctl/attach", 202, `{"name":"a","addr":"unix:/tmp/a.sock","phase":"attaching"}`)
	if err := a.Start("a", "/tmp/a.sock"); err != nil {
		t.Fatalf("Start 202 err = %v", err)
	}
	if c := d.last(t); c.Auth != "Bearer ctl" || string(c.Body) != `{"name":"a","addr":"unix:/tmp/a.sock"}` {
		t.Fatalf("attach call = %+v body=%s", c, c.Body)
	}
	d.answer("POST", "/api/ctl/attach", 409, `{"error":{"code":"already_running","message":"a is attached"}}`)
	if err := a.Start("a", "/tmp/a.sock"); !errors.Is(err, runner.ErrAlreadyRunning) {
		t.Fatalf("Start 409 err = %v, want ErrAlreadyRunning", err)
	}
	d.answer("POST", "/api/ctl/attach", 400, `{"error":{"code":"invalid_name","message":"bad name"}}`)
	if err := a.Start("A B", "/tmp/a.sock"); !errors.Is(err, runner.ErrInvalidName) {
		t.Fatalf("Start 400 err = %v, want ErrInvalidName", err)
	}
	d.answer("POST", "/api/ctl/attach", 403, `{"error":{"code":"control_disabled","message":"no control token"}}`)
	err := a.Start("a", "/tmp/a.sock")
	var de *xcclient.Error
	if !errors.As(err, &de) || de.Code != "control_disabled" || errors.Is(err, runner.ErrAlreadyRunning) {
		t.Fatalf("Start 403 err = %v", err)
	}

	d.answer("DELETE", "/api/ctl/instances/a", 200, `{"name":"a","detached":true}`)
	if err := a.Stop("a"); err != nil {
		t.Fatalf("Stop err = %v", err)
	}
	d.answer("DELETE", "/api/ctl/instances/a", 404, `{"error":{"code":"not_found","message":"nothing attached"}}`)
	if err := a.Stop("a"); err != nil {
		t.Fatalf("Stop 404 err = %v, want nil", err)
	}
}

// TestAdapterInspectCacheSingleFlight: concurrent Inspect calls share one
// daemon round trip, the answer is served from cache for 250 ms, and a
// stale entry refetches.
func TestAdapterInspectCacheSingleFlight(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, nil)
	st := runner.InspectState{Info: runner.Info{Name: "a", Sock: "/tmp/a.sock"}, Running: true, Phase: "live"}
	body, _ := json.Marshal(st)
	d.answer("GET", "/api/instances/a/inspect", 200, string(body))
	d.mu.Lock()
	d.delay = 30 * time.Millisecond
	d.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, ok := a.Inspect("a")
			if !ok || got.Name != "a" || got.Phase != "live" {
				t.Errorf("Inspect = %+v, %v", got, ok)
			}
		}()
	}
	wg.Wait()
	if n := d.hits.Load(); n != 1 {
		t.Fatalf("daemon hits after 8 concurrent inspects = %d, want 1", n)
	}
	if _, ok := a.Inspect("a"); !ok {
		t.Fatal("cached Inspect not ok")
	}
	if n := d.hits.Load(); n != 1 {
		t.Fatalf("daemon hits after a cached inspect = %d, want 1", n)
	}
	// Distinct names never share an entry.
	d.answer("GET", "/api/instances/b/inspect", 404, `{"error":{"code":"not_found","message":"no runner"}}`)
	if _, ok := a.Inspect("b"); ok {
		t.Fatal("Inspect of an unknown instance ok")
	}
	if n := d.hits.Load(); n != 2 {
		t.Fatalf("daemon hits = %d, want 2", n)
	}
	// Expiry: a stale entry refetches.
	a.inspect.now = func() time.Time { return time.Now().Add(time.Second) }
	if _, ok := a.Inspect("a"); !ok {
		t.Fatal("refetched Inspect not ok")
	}
	if n := d.hits.Load(); n != 3 {
		t.Fatalf("daemon hits after expiry = %d, want 3", n)
	}
}

// TestAdapterProxies: events / probe bytes come back verbatim, the map
// list is cached 2 s, SetSelection forwards the two names only, Status
// falls back to not-present on 404, Diagnostics unwraps host_runner.
func TestAdapterProxies(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, nil)

	events, _ := framedFixture(t, "events")
	d.answer("GET", "/api/instances/smoke1/events", 200, string(events))
	got, ok := a.EventsReply("smoke1", 7, []string{"death"})
	if !ok || string(got) != string(events) {
		t.Fatalf("EventsReply = %q, %v", got, ok)
	}
	if c := d.last(t); c.Auth != "Bearer feed" || c.Query != "since_tick=7&types=death" {
		t.Fatalf("events call = %+v", c)
	}
	d.answer("GET", "/api/instances/smoke1/probe", 504, `{"error":{"code":"probe_timeout","message":"slow"}}`)
	if _, ok := a.ProbeReply("smoke1"); ok {
		t.Fatal("ProbeReply ok on 504")
	}

	d.answer("GET", "/api/instances/smoke1/maps", 200, `{"available":true,"maps":[{"name":"Blood Gulch"}],"gametypes":[{"name":"Slayer"}]}`)
	hits := d.hits.Load()
	for i := 0; i < 3; i++ {
		if ml := a.AvailableMaps("smoke1"); !ml.Available || len(ml.Maps) != 1 || ml.Maps[0].Name != "Blood Gulch" {
			t.Fatalf("AvailableMaps = %+v", ml)
		}
	}
	if n := d.hits.Load() - hits; n != 1 {
		t.Fatalf("maps fetched %d times in 2 s, want 1", n)
	}

	d.answer("PUT", "/api/ctl/instances/smoke1/host/selection", 200, `{"instance":"smoke1","present":true,"authority":"runner","selected":true}`)
	if !a.SetSelection("smoke1", "Blood Gulch", 3, "Slayer", 5) {
		t.Fatal("SetSelection false")
	}
	if c := d.last(t); string(c.Body) != `{"map":"Blood Gulch","gametype":"Slayer"}` {
		t.Fatalf("selection body = %s, want the two names only", c.Body)
	}
	d.answer("PUT", "/api/ctl/instances/smoke1/host/selection", 409, `{"error":{"code":"not_in_carousel","message":"map"}}`)
	if a.SetSelection("smoke1", "Nope", 0, "Slayer", 0) {
		t.Fatal("SetSelection true on 409")
	}

	d.answer("GET", "/api/instances/smoke1/host", 404, `{"error":{"code":"hostrunner_disabled","message":"off"}}`)
	if st := a.Status("smoke1"); st.Present || st.Instance != "smoke1" {
		t.Fatalf("Status on 404 = %+v", st)
	}
	d.answer("GET", "/api/instances/smoke1/host", 200, `{"instance":"smoke1","present":true,"authority":"admin","machine_count":3}`)
	if st := a.Status("smoke1"); !st.Present || st.Authority != "admin" || st.MachineCount != 3 {
		t.Fatalf("Status = %+v", st)
	}
	d.answer("GET", "/api/instances/smoke1/diagnostics", 200, `{"host_runner":{"instance":"smoke1","present":true,"tick":9},"present":true,"readout":null,"health":null,"health_age_ms":0,"maps":{"available":false,"maps":[],"gametypes":[]}}`)
	if dg := a.Diagnostics("smoke1"); !dg.Present || dg.Tick != 9 {
		t.Fatalf("Diagnostics = %+v", dg)
	}
	d.answer("GET", "/api/instances/smoke1/readout", 404, `{"error":{"code":"no_readout","message":"none"}}`)
	if _, ok := a.Readout("smoke1"); ok {
		t.Fatal("Readout ok on 404")
	}
	d.answer("GET", "/api/instances/smoke1/health", 200, `{"status":"ok","observed_hz":29.5,"expected_hz":30,"ratio":0.98,"samples":12,"confident":true}`)
	if hh, ok := a.HostHealth("smoke1"); !ok || hh.ExpectedHz != 30 || !hh.Confident {
		t.Fatalf("HostHealth = %+v, %v", hh, ok)
	}
}

// TestAdapterMirrorViews: the scraperiface reads come from the mirror —
// List / InstanceState / Membership / the four JoinReplay* (raw bytes as
// the daemon framed them).
func TestAdapterMirrorViews(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, nil)
	if a.JoinReplayMessages() != nil || len(a.List()) != 0 || len(a.Membership()) != 0 {
		t.Fatal("empty mirror answered something")
	}
	gameRaw, gameEnv := framedFixture(t, "game")
	a.client.Mirror().Store(gameEnv.Instance, gameEnv.Type, gameRaw, &gameEnv)
	storeFixture(t, a, "scenario")
	sumRaw, sumEnv := framedFixture(t, "summary")
	var sp wire.SummaryPayload
	if err := json.Unmarshal(sumEnv.Data, &sp); err != nil {
		t.Fatal(err)
	}
	a.client.Mirror().StoreSummary(sumRaw, &sp)

	if infos := a.List(); len(infos) != 1 || infos[0].Name != "smoke1" {
		t.Fatalf("List = %+v", infos)
	}
	if st, ok := a.InstanceState("smoke1"); !ok || st.Name != "smoke1" {
		t.Fatalf("InstanceState = %+v, %v", st, ok)
	}
	mem := a.Membership()
	if len(mem) != 1 || mem[0].Container != "smoke1" || len(mem[0].Identities) == 0 {
		t.Fatalf("Membership = %+v", mem)
	}
	if got := a.JoinReplayForInstanceClass("smoke1", wire.ClassGame); len(got) != 1 || string(got[0]) != string(gameRaw) {
		t.Fatalf("JoinReplayForInstanceClass = %d frame(s)", len(got))
	}
	if got := a.JoinReplayForInstance("smoke1"); len(got) != 2 {
		t.Fatalf("JoinReplayForInstance = %d frame(s), want game + scenario", len(got))
	}
	if got := a.JoinReplayForHostAll(); len(got) != 1 || string(got[0]) != string(sumRaw) {
		t.Fatalf("JoinReplayForHostAll = %d frame(s)", len(got))
	}
	if got := a.JoinReplayMessages(); len(got) != 2 {
		t.Fatalf("JoinReplayMessages = %d frame(s)", len(got))
	}
	if n := d.hits.Load(); n != 0 {
		t.Fatalf("mirror reads hit the daemon %d time(s)", n)
	}
}

// TestValidateUpstreamURL: XC_SCRAPER_URL must be exactly
// http(s)://host[:port] (§11).
func TestValidateUpstreamURL(t *testing.T) {
	for _, ok := range []string{"http://127.0.0.1:8990", "https://scraper.lan", "http://[::1]:8990/", " http://h:1 "} {
		if err := ValidateUpstreamURL(ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ws://h:1", "wss://h:1", "http://h:1/api", "http://h:1/api/ws", "h:1",
		"127.0.0.1:8990", "http://", "http://u:p@h:1", "http://h:1?token=x", "http://h:1#f", "ftp://h"} {
		err := ValidateUpstreamURL(bad)
		if !errors.Is(err, ErrBadUpstreamURL) {
			t.Errorf("%q: err = %v, want ErrBadUpstreamURL", bad, err)
		}
	}
	if ErrBadUpstreamURL.Error() != "leaguescraper: XC_SCRAPER_URL must be http(s)://host[:port]" {
		t.Fatalf("message = %q", ErrBadUpstreamURL.Error())
	}
}

// helloInstanceNames flattens a payload's instance list.
func helloInstanceNames(p wire.HelloPayload) []string {
	names := make([]string, 0, len(p.Instances))
	for _, in := range p.Instances {
		names = append(names, in.Name)
	}
	return names
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestHelloBeforeUpstream: with no daemon hello seen yet the downstream
// hello carries this build's protocol version + wire.AllClasses() and
// instances [] (never null, never classes: []).
func TestHelloBeforeUpstream(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, func() bool { return false })
	if a.UpstreamHelloSeen() {
		t.Fatal("hello seen before any frame")
	}
	p := a.HelloPayloadFor(authz.Superuser("su"))
	if p.ProtocolVersion != wire.ProtocolVersion {
		t.Fatalf("protocol_version = %d, want %d", p.ProtocolVersion, wire.ProtocolVersion)
	}
	if !sameStrings(p.Classes, wire.AllClasses()) {
		t.Fatalf("classes = %v, want wire.AllClasses()", p.Classes)
	}
	if p.Instances == nil || len(p.Instances) != 0 {
		t.Fatalf("instances = %#v, want []", p.Instances)
	}
	if p.ServerTime.IsZero() {
		t.Fatal("server_time zero")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	if string(m["instances"]) != "[]" {
		t.Fatalf("instances JSON = %s, want []", m["instances"])
	}

	// Non-hello frames leave the defaults alone; a hello takes over.
	gameRaw, gameEnv := framedFixture(t, "game")
	a.OnFrame(xcclient.Frame{Raw: gameRaw, Env: &gameEnv})
	if a.UpstreamHelloSeen() {
		t.Fatal("game frame counted as hello")
	}
	hp := wire.HelloPayload{ProtocolVersion: 3, ServerTime: time.Now(), Classes: []string{"game", "tick"}}
	env := wire.Envelope{V: wire.ProtocolVersion, Type: wire.ClassHello}
	env.Data, _ = json.Marshal(hp)
	a.OnFrame(xcclient.Frame{Env: &env})
	if !a.UpstreamHelloSeen() {
		t.Fatal("upstream hello not recorded")
	}
	p = a.HelloPayloadFor(authz.Superuser("su"))
	if p.ProtocolVersion != 3 || !sameStrings(p.Classes, []string{"game", "tick"}) {
		t.Fatalf("after upstream hello: version %d classes %v", p.ProtocolVersion, p.Classes)
	}
	p.Classes[0] = "mutated"
	if q := a.HelloPayloadFor(authz.Superuser("su")); q.Classes[0] != "game" {
		t.Fatal("HelloPayloadFor shares its classes slice")
	}
}

// TestHelloPayloadForFiltersByPrincipal (relocated from wireadapter_test.go
// for the wire-mode adapter): the instances list is authz.JoinableInstances
// over the mirror's instance set — admins / users / machine keys see every
// live instance, a bound spectator / device / console door only the one it
// is bound to and only while live, everyone else [] — with each instance's
// mirrored started_at.
func TestHelloPayloadForFiltersByPrincipal(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, func() bool { return false })
	startA := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	startB := startA.Add(time.Minute)
	a.client.Mirror().SetInstance("pod-a", startA, false)
	a.client.Mirror().SetInstance("pod-b", startB, false)

	bound := func(kind authz.Kind, instance string) authz.Principal {
		p := authz.Principal{Kind: kind, ID: string(kind) + "-1", Scopes: authz.CanonScopes([]string{"room.join:*"})}
		if instance != "" {
			p.Bound = map[string]string{"instance": instance}
		}
		return p
	}
	tests := []struct {
		name string
		p    authz.Principal
		want []string
	}{
		{"pb_user", authz.Principal{Kind: authz.KindPBUser, ID: "u1", UserID: "u1"}, []string{"pod-a", "pod-b"}},
		{"superuser", authz.Superuser("su"), []string{"pod-a", "pod-b"}},
		{"machine", authz.Principal{Kind: authz.KindMachine, ID: "k1"}, []string{"pod-a", "pod-b"}},
		{"spectator bound to live instance", bound(authz.KindSpectator, "pod-b"), []string{"pod-b"}},
		{"device bound to live instance", bound(authz.KindDevice, "pod-a"), []string{"pod-a"}},
		{"anonymous console door bound", authz.Anonymous("pod-a", nil), []string{"pod-a"}},
		{"spectator bound to an instance that is not live", bound(authz.KindSpectator, "pod-z"), []string{}},
		{"device without a binding", bound(authz.KindDevice, ""), []string{}},
		{"nobody", authz.Nobody(), []string{}},
		{"discord", authz.Principal{Kind: authz.KindDiscord, ID: "d1"}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := a.HelloPayloadFor(tc.p)
			if p.Instances == nil {
				t.Fatal("instances = nil, want a non-nil slice so JSON marshals as []")
			}
			if got := helloInstanceNames(p); !sameStrings(got, tc.want) {
				t.Fatalf("instances = %v, want %v", got, tc.want)
			}
			for _, in := range p.Instances {
				want := startA
				if in.Name == "pod-b" {
					want = startB
				}
				if !in.StartedAt.Equal(want) {
					t.Fatalf("%s started_at = %v, want %v", in.Name, in.StartedAt, want)
				}
			}
			if p.ProtocolVersion != wire.ProtocolVersion || !sameStrings(p.Classes, wire.AllClasses()) {
				t.Fatalf("protocol fields changed by the filter: version %d classes %v", p.ProtocolVersion, p.Classes)
			}
		})
	}
	if n := d.hits.Load(); n != 0 {
		t.Fatalf("hello hit the daemon %d time(s)", n)
	}
}

// TestAdapterSendHelloOn: SendHelloOn sends exactly one frame — a
// wire.Message{Type:"scraper"} with no room wrapping a hello envelope whose
// instances are the principal's joinable set.
func TestAdapterSendHelloOn(t *testing.T) {
	d := newDaemonStub(t)
	a := newTestAdapter(t, d, func() bool { return false })
	a.client.Mirror().SetInstance("pod-a", time.Now(), false)
	a.client.Mirror().SetInstance("pod-b", time.Now(), false)

	var sent [][]byte
	a.SendHelloOn(func(data []byte) { sent = append(sent, data) }, authz.Anonymous("pod-b", nil))
	if len(sent) != 1 {
		t.Fatalf("sent %d frame(s), want 1", len(sent))
	}
	var msg wire.Message
	if err := json.Unmarshal(sent[0], &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != wire.TypeScraper || msg.Room != "" {
		t.Fatalf("frame type=%q room=%q, want scraper with no room", msg.Type, msg.Room)
	}
	var env wire.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type != wire.ClassHello || env.Instance != "" || env.Seq != 0 || env.V != wire.ProtocolVersion {
		t.Fatalf("envelope = %+v", env)
	}
	var payload wire.HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if got := helloInstanceNames(payload); !sameStrings(got, []string{"pod-b"}) {
		t.Fatalf("hello instances = %v, want [pod-b]", got)
	}
	if !sameStrings(payload.Classes, wire.AllClasses()) {
		t.Fatalf("classes = %v", payload.Classes)
	}
}
