package websocket

import (
	"encoding/json"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/handlers"
)

// principalOfKind builds one representative connection principal per
// authz kind — the shape pb.ResolveWS would hand the Hub for that
// credential (bound keys bound to pod-a with a matching room scope, the
// console door as Nobody, discord and internal never resolved on a socket
// but still dispatched fail-closed).
func principalOfKind(k authz.Kind) authz.Principal {
	switch k {
	case authz.KindPBUser:
		return userPrincipal("u1")
	case authz.KindSuperuser:
		return authz.Superuser("su")
	case authz.KindMachine:
		return machineKey("k1")
	case authz.KindSpectator, authz.KindDevice:
		return authz.Principal{
			Kind:   k,
			ID:     string(k) + "-1",
			Scopes: authz.CanonScopes([]string{"room.join:host:pod-a:*", "scraper.*"}),
			Bound:  map[string]string{"instance": "pod-a"},
		}
	case authz.KindAnonymous:
		return authz.Nobody()
	case authz.KindInternal:
		return authz.Internal("test")
	default:
		return authz.Principal{Kind: k, ID: string(k) + "-1"}
	}
}

// errorCode decodes one queued frame as the Hub's error envelope and
// returns its code; "" when the frame is not an error envelope.
func errorCode(t *testing.T, raw string) string {
	t.Helper()
	var msg Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("frame %q: %v", raw, err)
	}
	if msg.Type != TypeError {
		return ""
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("error payload %q: %v", msg.Payload, err)
	}
	if payload.Code == "" || payload.Message == "" {
		t.Fatalf("error frame %q: code and message must both be set", raw)
	}
	return payload.Code
}

// connected reports whether the client is still in the hub's client set.
func connected(h *Hub, c *Client) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[c]
}

// TestDispatchUnknownTypeIsErrorForAllKinds: a message type with no
// registered handler is never fanned out — the pre-authz Hub broadcast it
// to every client — but answered with a single error frame to the sender
// alone, for every principal kind, and the sender stays connected. Kinds
// the ws.send whitelist admits see unknown_type; kinds it refuses are
// stopped by the whitelist first (forbidden), and the outcome is the same:
// an error, nothing broadcast. The Hub's own routing constants (broadcast
// / room / direct) are unregistered types when they arrive from a client.
func TestDispatchUnknownTypeIsErrorForAllKinds(t *testing.T) {
	types := []string{"no_such_type", TypeBroadcast, TypeRoom, TypeDirect, "join_rooms"}
	for _, kind := range authz.Kinds() {
		for _, msgType := range types {
			t.Run(string(kind)+"/"+msgType, func(t *testing.T) {
				if _, ok := handlers.Get(msgType); ok {
					t.Fatalf("%q has a handler; pick an unregistered type", msgType)
				}
				h := NewHub(nil)
				h.deps = &authztest.FakeDeps{}
				sender := addTestClient(h, principalOfKind(kind), "public:lobby")
				peer := addTestClient(h, userPrincipal("peer"), "public:lobby")

				h.dispatch(incomingMsg{
					msg:    Message{Type: msgType, Room: "public:lobby", Target: "peer", Payload: json.RawMessage(`{"x":1}`)},
					sender: sender,
				})

				got := drainSend(sender)
				if len(got) != 1 {
					t.Fatalf("sender got %d frames %q, want exactly one error", len(got), got)
				}
				code := errorCode(t, got[0])
				want := "forbidden"
				if authz.WSSendAllowed(kind, msgType) {
					want = "unknown_type"
				}
				if code != want {
					t.Fatalf("error code = %q, want %q", code, want)
				}
				if peerGot := drainSend(peer); len(peerGot) != 0 {
					t.Fatalf("peer received %q, want nothing (unknown types must never broadcast)", peerGot)
				}
				if !connected(h, sender) {
					t.Fatal("sender was dropped; an unknown type is an error, not a disconnect")
				}
			})
		}
	}
}

// TestDispatchWSSendWhitelist pins the per-kind ws.send whitelist at the
// Hub: the check runs before the handler lookup, so a refused kind never
// reaches a registered handler (no join, no reply), gets error{forbidden}
// and stays connected; an admitted kind reaches the handler (join_room's
// own decision then applies). Every registered handler type is covered
// for every kind, driven off authz.WSSendAllowed so the two cannot drift.
func TestDispatchWSSendWhitelist(t *testing.T) {
	msgTypes := []string{"join_room", "leave_room", "request_state", "request_events", "request_probe"}
	for _, msgType := range msgTypes {
		if _, ok := handlers.Get(msgType); !ok {
			t.Fatalf("%q has no handler registered", msgType)
		}
	}
	for _, kind := range authz.Kinds() {
		for _, msgType := range msgTypes {
			t.Run(string(kind)+"/"+msgType, func(t *testing.T) {
				h := NewHub(nil)
				h.deps = &authztest.FakeDeps{}
				sender := addTestClient(h, principalOfKind(kind))

				// join_room on a public room is the one request every
				// kind but discord would be admitted to by authz — so a
				// refusal here can only be the whitelist.
				h.dispatch(incomingMsg{msg: Message{Type: msgType, Room: "public:lobby"}, sender: sender})

				got := drainSend(sender)
				joined := false
				h.mu.RLock()
				if members := h.rooms["public:lobby"]; members != nil {
					joined = members[sender]
				}
				h.mu.RUnlock()

				if !authz.WSSendAllowed(kind, msgType) {
					if len(got) != 1 || errorCode(t, got[0]) != "forbidden" {
						t.Fatalf("refused kind got %q, want one error{forbidden}", got)
					}
					if joined {
						t.Fatal("refused kind reached join_room and was joined")
					}
					if !connected(h, sender) {
						t.Fatal("refused kind was dropped; the whitelist must not disconnect")
					}
					return
				}
				for _, frame := range got {
					if errorCode(t, frame) == "forbidden" {
						t.Fatalf("admitted kind got error{forbidden} from the Hub: %q", got)
					}
				}
				if msgType == "join_room" && kind != authz.KindDiscord && !joined {
					t.Fatalf("admitted kind was not joined to public:lobby (frames %q)", got)
				}
			})
		}
	}
}

// TestDispatchInternalMessagesRoute: the Hub's own Broadcast / SendToRoom
// / SendToUser API queues sender-less messages; those bypass the whitelist
// and the handler registry and are routed by type. A client can never
// forge one — every client message carries its sender (see the two tests
// above).
func TestDispatchInternalMessagesRoute(t *testing.T) {
	h := NewHub(nil)
	h.deps = &authztest.FakeDeps{}
	inRoom := addTestClient(h, machineKey("k1"), "host:pod-a:tick")
	user := addTestClient(h, userPrincipal("u1"))
	other := addTestClient(h, authz.Nobody())

	h.dispatch(incomingMsg{msg: Message{Type: TypeRoom, Room: "host:pod-a:tick", Payload: json.RawMessage(`"r"`)}})
	h.dispatch(incomingMsg{msg: Message{Type: TypeDirect, Target: "u1", Payload: json.RawMessage(`"d"`)}})
	h.dispatch(incomingMsg{msg: Message{Type: TypeBroadcast, Payload: json.RawMessage(`"b"`)}})

	if got := drainSend(inRoom); len(got) != 2 {
		t.Errorf("room member got %q, want the room message and the broadcast", got)
	}
	if got := drainSend(user); len(got) != 2 {
		t.Errorf("user got %q, want the direct message and the broadcast", got)
	}
	if got := drainSend(other); len(got) != 1 {
		t.Errorf("bystander got %q, want only the broadcast", got)
	}
}
