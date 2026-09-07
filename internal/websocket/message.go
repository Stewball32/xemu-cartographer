package websocket

import "github.com/xemu-cartographer/xc-scraper/wire"

// Message type constants for routing. Owned by the wire contract package
// (xc-scraper/wire/message.go); re-declared here with identical values so
// every existing caller keeps compiling.
const (
	TypeBroadcast = wire.TypeBroadcast
	TypeRoom      = wire.TypeRoom
	TypeDirect    = wire.TypeDirect
	TypeJoinRoom  = wire.TypeJoinRoom
	TypeLeaveRoom = wire.TypeLeaveRoom
	TypeError     = wire.TypeError
	// TypeRoomLeft is a server-initiated frame: the periodic re-resolve
	// re-decided room.join for a room the connection held and it no longer
	// passes, so the Hub dropped the membership. One frame per room, sent
	// before the membership goes; payload {"reason":"forbidden"}.
	TypeRoomLeft = wire.TypeRoomLeft
	// TypeScraper frames every scraper envelope the league emitter fans out
	// to a host:* room (internal/leaguescraper/emitter.go).
	TypeScraper = wire.TypeScraper
)

// Message is the wire format for all WebSocket communication.
// Hub inspects Type to decide routing. Payload is opaque project-specific data.
//
// Alias of wire.Message — the outer transport layer of the xc-scraper wire
// contract — so the flagship hub and the scraper producers share one type
// identity.
type Message = wire.Message
