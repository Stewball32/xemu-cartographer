package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/xemu-cartographer/xc-scraper/scraper"
)

// helloNames flattens a payload's instance list for comparison.
func helloNames(p HelloPayload) []string {
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
func TestHelloPayloadForFiltersByPrincipal(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	started := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{"pod-b", "pod-a"} {
		r := newRunner(name, "/tmp/"+name, "host:"+name, nil, nil, nil)
		defer r.cancel()
		r.cache.StartedAt = started
		m.runners[name] = r
	}

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
			p := m.HelloPayloadFor(tc.p)
			if p.Instances == nil {
				t.Fatal("instances = nil, want a non-nil slice so JSON marshals as []")
			}
			got := helloNames(p)
			if len(got) != len(tc.want) {
				t.Fatalf("instances = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("instances = %v, want %v", got, tc.want)
				}
			}
			for _, inst := range p.Instances {
				if !inst.StartedAt.Equal(started) {
					t.Fatalf("%s started_at = %v, want %v", inst.Name, inst.StartedAt, started)
				}
			}
			if p.ProtocolVersion != scraper.ProtocolVersion || len(p.Classes) != len(allClasses) {
				t.Fatalf("protocol fields changed by the filter: version %d classes %v", p.ProtocolVersion, p.Classes)
			}
		})
	}
}

// TestSendHelloOnUsesFilteredPayload: the bytes SendHelloOn enqueues carry
// the narrowed instance list — a bound key never learns the names of the
// other live instances from the handshake.
func TestSendHelloOnUsesFilteredPayload(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	for _, name := range []string{"pod-a", "pod-b"} {
		r := newRunner(name, "/tmp/"+name, "host:"+name, nil, nil, nil)
		defer r.cancel()
		m.runners[name] = r
	}

	var got []byte
	m.SendHelloOn(func(data []byte) { got = data }, authz.Principal{
		Kind:  authz.KindSpectator,
		ID:    "s1",
		Bound: map[string]string{"instance": "pod-b"},
	})
	if len(got) == 0 {
		t.Fatal("SendHelloOn: send received empty bytes")
	}

	var msg websocket.Message
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	var payload HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal hello payload: %v", err)
	}
	if names := helloNames(payload); len(names) != 1 || names[0] != "pod-b" {
		t.Fatalf("hello instances = %v, want [pod-b]", names)
	}
}
