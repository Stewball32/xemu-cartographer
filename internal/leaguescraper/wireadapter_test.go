package leaguescraper

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/scraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// These tests own every wire.Message-framing assertion that used to live in
// the manager package (wire_test.go, hello_test.go, hello_filter_test.go,
// events_test.go, aggregator_test.go) before step 7 part 3c moved the frame
// + room choice into the WireAdapter. The manager cannot be populated with
// runners from outside its package (that needs an xemu instance), so the
// room / frame assertions run against the framing helpers with synthetic
// runner.Reply values, and the Manager-facing methods are exercised through
// an empty Manager (whose summary replay and hello are real).

// envelopeBytes marshals a minimal envelope for class/instance.
func envelopeBytes(t *testing.T, class, instance string, tick uint32) []byte {
	t.Helper()
	env := scraper.MakeEnvelope(class, instance, 1, tick, map[string]any{"k": "v"})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	return b
}

// decodeFramed unwraps a wire.Message (the bytes a WS client receives) to
// the message + inner envelope.
func decodeFramed(t *testing.T, data []byte) (websocket.Message, scraper.Envelope) {
	t.Helper()
	var msg websocket.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	return msg, env
}

// TestReplyRoomTable pins the room every reply class is framed for: the
// per-class room for state classes (what the pre-3c join replay used), the
// legacy bare host:<inst> for the events + probe request/reply channels
// (protocol bug 4 — preserved on purpose), and host:summary for the
// cross-instance summary.
func TestReplyRoomTable(t *testing.T) {
	cases := []struct {
		rep  runner.Reply
		want string
	}{
		{runner.Reply{Instance: "bravo", Class: "tick"}, "host:bravo:tick"},
		{runner.Reply{Instance: "alpha", Class: "game"}, "host:alpha:game"},
		{runner.Reply{Instance: "alpha", Class: wire.ClassGameFiltered}, "host:alpha:game_filtered"},
		{runner.Reply{Instance: "alpha", Class: wire.ClassPreviousGame}, "host:alpha:previous_game"},
		{runner.Reply{Instance: "alpha", Class: wire.ClassEvents}, "host:alpha"},
		{runner.Reply{Instance: "alpha", Class: wire.ClassProbe}, "host:alpha"},
		{runner.Reply{Instance: "", Class: wire.ClassSummary}, wire.SummaryRoom},
	}
	for _, tc := range cases {
		got, ok := replyRoom(tc.rep)
		if !ok || got != tc.want {
			t.Fatalf("replyRoom(%+v) = (%q, %v), want (%q, true)", tc.rep, got, ok, tc.want)
		}
	}
	if room, ok := replyRoom(runner.Reply{Instance: "alpha", Class: "nope"}); ok {
		t.Fatalf("replyRoom(unknown class) = (%q, true), want unroutable", room)
	}
}

// TestFrameReplyWrapsEnvelopeUnchanged: the framed bytes are
// wire.Message{type:"scraper", room:<room>, payload:<envelope>} with the
// envelope byte-identical inside — the same three-layer shape the manager
// framed before 3c (tick → host:bravo:tick, events → host:alpha).
func TestFrameReplyWrapsEnvelopeUnchanged(t *testing.T) {
	env := envelopeBytes(t, "tick", "bravo", 1234)
	data, ok := frameReply(runner.Reply{Instance: "bravo", Class: "tick", Envelope: env})
	if !ok {
		t.Fatal("frameReply: ok=false")
	}
	msg, inner := decodeFramed(t, data)
	if msg.Type != "scraper" {
		t.Fatalf("msg.type = %q, want %q", msg.Type, "scraper")
	}
	if msg.Room != "host:bravo:tick" {
		t.Fatalf("tick room = %q, want %q", msg.Room, "host:bravo:tick")
	}
	if string(msg.Payload) != string(env) {
		t.Fatalf("payload bytes changed by framing:\n got %s\nwant %s", msg.Payload, env)
	}
	if inner.Type != "tick" || inner.Instance != "bravo" || inner.V != scraper.ProtocolVersion || inner.Tick != 1234 {
		t.Fatalf("inner envelope = %+v", inner)
	}

	env = envelopeBytes(t, wire.ClassEvents, "alpha", 100)
	data, ok = frameReply(runner.Reply{Instance: "alpha", Class: wire.ClassEvents, Envelope: env})
	if !ok {
		t.Fatal("frameReply(events): ok=false")
	}
	msg, inner = decodeFramed(t, data)
	if msg.Room != "host:alpha" {
		t.Fatalf("events msg.room = %q, want %q", msg.Room, "host:alpha")
	}
	if inner.Type != wire.ClassEvents || inner.Instance != "alpha" || inner.Tick != 100 {
		t.Fatalf("events inner envelope = %+v", inner)
	}
}

// TestFrameRepliesPreservesNilAndOrder: a nil reply list (runner does not
// exist) stays nil so the WS handlers' "nothing to replay" branch is
// unchanged; a populated list is framed in order with unroutable entries
// dropped.
func TestFrameRepliesPreservesNilAndOrder(t *testing.T) {
	if got := frameReplies(nil); got != nil {
		t.Fatalf("frameReplies(nil) = %v, want nil", got)
	}
	if got := frameReplies([]runner.Reply{}); got == nil || len(got) != 0 {
		t.Fatalf("frameReplies(empty) = %v, want empty non-nil", got)
	}
	reps := []runner.Reply{
		{Instance: "a", Class: "xbox", Envelope: envelopeBytes(t, "xbox", "a", 0)},
		{Instance: "a", Class: "bogus", Envelope: envelopeBytes(t, "bogus", "a", 0)},
		{Instance: "a", Class: "game", Envelope: envelopeBytes(t, "game", "a", 0)},
	}
	got := frameReplies(reps)
	if len(got) != 2 {
		t.Fatalf("frameReplies: %d messages, want 2 (bogus dropped)", len(got))
	}
	m0, _ := decodeFramed(t, got[0])
	m1, _ := decodeFramed(t, got[1])
	if m0.Room != "host:a:xbox" || m1.Room != "host:a:game" {
		t.Fatalf("rooms = [%q %q], want [host:a:xbox host:a:game]", m0.Room, m1.Room)
	}
}

// TestAdapterJoinReplayForHostAll: through a real (empty) Manager the
// summary replay is one wire.Message addressed to host:summary carrying a
// "summary" envelope — what TestAggregatorJoinReplay asserted pre-3c.
func TestAdapterJoinReplayForHostAll(t *testing.T) {
	m := runner.New(runner.Options{})
	defer m.Close()
	a := NewWireAdapter(m, nil)

	out := a.JoinReplayForHostAll()
	if len(out) != 1 {
		t.Fatalf("JoinReplayForHostAll: %d messages, want 1", len(out))
	}
	msg, env := decodeFramed(t, out[0])
	if msg.Type != "scraper" || msg.Room != wire.SummaryRoom {
		t.Fatalf("summary frame = type %q room %q, want scraper/%q", msg.Type, msg.Room, wire.SummaryRoom)
	}
	if env.Type != wire.ClassSummary || env.Instance != "" {
		t.Fatalf("summary envelope = type %q instance %q", env.Type, env.Instance)
	}

	// The per-instance paths through an empty manager: unknown runner → nil
	// (join_room / request_state skip the replay), all-runners → empty.
	if got := a.JoinReplayForInstance("nope"); got != nil {
		t.Fatalf("JoinReplayForInstance(unknown) = %v, want nil", got)
	}
	if got := a.JoinReplayForInstanceClass("nope", "tick"); got != nil {
		t.Fatalf("JoinReplayForInstanceClass(unknown) = %v, want nil", got)
	}
	if got := a.JoinReplayMessages(); len(got) != 0 {
		t.Fatalf("JoinReplayMessages(empty) = %d messages, want 0", len(got))
	}
	if b, ok := a.EventsReply("nope", 0, nil); ok || b != nil {
		t.Fatalf("EventsReply(unknown) = (%q, %v), want (nil, false)", b, ok)
	}
	if b, ok := a.ProbeReply("nope"); ok || b != nil {
		t.Fatalf("ProbeReply(unknown) = (%q, %v), want (nil, false)", b, ok)
	}
}

// TestHelloMessageBytesRoundtrip: the connect-hook bytes unmarshal back
// through the expected three layers (websocket.Message → scraper.Envelope →
// HelloPayload), carry type "scraper" with NO room (hello is not per-room),
// the protocol version, empty instance and tick=0, and the payload's
// instance list verbatim.
func TestHelloMessageBytesRoundtrip(t *testing.T) {
	started := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	in := wire.HelloPayload{
		ProtocolVersion: scraper.ProtocolVersion,
		ServerTime:      time.Now(),
		Classes:         wire.AllClasses(),
		Instances:       []wire.HelloInstance{{Name: "alpha", StartedAt: started}},
	}
	bytes, ok := helloMessageBytes(in)
	if !ok {
		t.Fatal("helloMessageBytes: ok=false")
	}

	msg, env := decodeFramed(t, bytes)
	if msg.Type != "scraper" {
		t.Fatalf("msg.type = %q, want %q", msg.Type, "scraper")
	}
	if msg.Room != "" {
		t.Fatalf("msg.room = %q, want empty (hello is not per-room)", msg.Room)
	}
	if env.V != scraper.ProtocolVersion {
		t.Fatalf("env.v = %d, want %d", env.V, scraper.ProtocolVersion)
	}
	if env.Type != wire.ClassHello {
		t.Fatalf("env.type = %q, want %q", env.Type, wire.ClassHello)
	}
	if env.Instance != "" {
		t.Fatalf("env.instance = %q, want empty", env.Instance)
	}
	if env.Tick != 0 {
		t.Fatalf("env.tick = %d, want 0", env.Tick)
	}

	var payload wire.HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal HelloPayload: %v", err)
	}
	if payload.ProtocolVersion != scraper.ProtocolVersion {
		t.Fatalf("payload.protocol_version = %d, want %d", payload.ProtocolVersion, scraper.ProtocolVersion)
	}
	if len(payload.Instances) != 1 || payload.Instances[0].Name != "alpha" || !payload.Instances[0].StartedAt.Equal(started) {
		t.Fatalf("payload.instances = %+v, want one entry [alpha @ %v]", payload.Instances, started)
	}
	if len(payload.Classes) != len(wire.AllClasses()) {
		t.Fatalf("payload.classes = %v, want all %d classes", payload.Classes, len(wire.AllClasses()))
	}
}

// helloNames flattens a payload's instance list for comparison.
func helloNames(p wire.HelloPayload) []string {
	out := make([]string, 0, len(p.Instances))
	for _, inst := range p.Instances {
		out = append(out, inst.Name)
	}
	return out
}

// TestHelloPayloadForFiltersByPrincipal is the W-1 hello narrowing: users,
// superusers and machine keys are told about every live instance; a
// spectator / device key or the console door sees only the instance it is
// bound to, and nothing when that instance is not live or the binding is
// missing. The instance list is always a JSON array, never null, and the
// protocol fields are not identity-dependent.
//
// The live instance list is what the manager reports; an empty manager
// reports none, so the per-principal narrowing over a populated list is
// exercised through the same keep function the adapter installs
// (authz.JoinableInstances), and the adapter method itself is checked to
// yield [] (not nil) for every principal.
func TestHelloPayloadForFiltersByPrincipal(t *testing.T) {
	m := runner.New(runner.Options{})
	defer m.Close()
	a := NewWireAdapter(m, nil)

	live := []string{"pod-a", "pod-b"}
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
			got := authz.JoinableInstances(tc.p, live)
			if got == nil {
				t.Fatal("JoinableInstances = nil, want a non-nil slice")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("instances = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("instances = %v, want %v", got, tc.want)
				}
			}

			p := a.HelloPayloadFor(tc.p)
			if p.Instances == nil {
				t.Fatal("HelloPayloadFor instances = nil, want a non-nil slice so JSON marshals as []")
			}
			if len(p.Instances) != 0 {
				t.Fatalf("HelloPayloadFor over an empty manager = %v, want []", helloNames(p))
			}
			if p.ProtocolVersion != scraper.ProtocolVersion || len(p.Classes) != len(wire.AllClasses()) {
				t.Fatalf("protocol fields changed by the filter: version %d classes %v", p.ProtocolVersion, p.Classes)
			}
		})
	}
}

// TestSendHelloOn: SendHelloOn invokes the send function exactly once with
// the same bytes helloMessageBytes(HelloPayloadFor(p)) would have returned
// (modulo the captured server_time, which advances per call), framed as a
// websocket.Message wrapping a hello envelope.
func TestSendHelloOn(t *testing.T) {
	m := runner.New(runner.Options{})
	defer m.Close()
	a := NewWireAdapter(m, nil)

	calls := 0
	var got []byte
	a.SendHelloOn(func(data []byte) {
		calls++
		got = data
	}, authz.Superuser("su"))

	if calls != 1 {
		t.Fatalf("SendHelloOn: send called %d times, want 1", calls)
	}
	if len(got) == 0 {
		t.Fatal("SendHelloOn: send received empty bytes")
	}
	msg, env := decodeFramed(t, got)
	if msg.Type != "scraper" {
		t.Fatalf("msg.type = %q, want %q", msg.Type, "scraper")
	}
	if msg.Room != "" {
		t.Fatalf("msg.room = %q, want empty", msg.Room)
	}
	if env.Type != wire.ClassHello {
		t.Fatalf("env.type = %q, want %q", env.Type, wire.ClassHello)
	}
	var payload wire.HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal hello payload: %v", err)
	}
	if payload.Instances == nil || len(payload.Instances) != 0 {
		t.Fatalf("hello instances = %v, want [] for an empty manager", payload.Instances)
	}
	if payload.ProtocolVersion != wire.ProtocolVersion {
		t.Fatalf("protocol_version = %d, want %d", payload.ProtocolVersion, wire.ProtocolVersion)
	}
}

// TestSendHelloOnUsesFilteredPayload: the bytes SendHelloOn enqueues carry
// the narrowed instance list — a bound key never learns the names of the
// other live instances from the handshake. The narrowing is the same
// helloMessageBytes(filter(payload)) pipeline SendHelloOn runs, applied to
// a two-instance payload (the manager cannot be populated from here).
func TestSendHelloOnUsesFilteredPayload(t *testing.T) {
	full := wire.HelloPayload{
		ProtocolVersion: scraper.ProtocolVersion,
		ServerTime:      time.Now(),
		Classes:         wire.AllClasses(),
		Instances: []wire.HelloInstance{
			{Name: "pod-a", StartedAt: time.Now()},
			{Name: "pod-b", StartedAt: time.Now()},
		},
	}
	p := authz.Principal{
		Kind:  authz.KindSpectator,
		ID:    "s1",
		Bound: map[string]string{"instance": "pod-b"},
	}
	allowed := map[string]bool{}
	for _, name := range authz.JoinableInstances(p, helloNames(full)) {
		allowed[name] = true
	}
	kept := make([]wire.HelloInstance, 0, 1)
	for _, inst := range full.Instances {
		if allowed[inst.Name] {
			kept = append(kept, inst)
		}
	}
	full.Instances = kept

	got, ok := helloMessageBytes(full)
	if !ok || len(got) == 0 {
		t.Fatal("helloMessageBytes: send received empty bytes")
	}
	_, env := decodeFramed(t, got)
	var payload wire.HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal hello payload: %v", err)
	}
	if names := helloNames(payload); len(names) != 1 || names[0] != "pod-b" {
		t.Fatalf("hello instances = %v, want [pod-b]", names)
	}

	// And through the adapter for a bound spectator over an empty manager:
	// exactly one send, with an empty (not null) instance list.
	m := runner.New(runner.Options{})
	defer m.Close()
	a := NewWireAdapter(m, nil)
	calls := 0
	a.SendHelloOn(func([]byte) { calls++ }, p)
	if calls != 1 {
		t.Fatalf("SendHelloOn(bound spectator): send called %d times, want 1", calls)
	}
}
