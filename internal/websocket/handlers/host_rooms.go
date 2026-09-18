package handlers

import "github.com/xemu-cartographer/xemu-cartographer/internal/authz"

// instanceOfRoom returns the instance a per-instance host room addresses
// ("host:<inst>" or "host:<inst>:<class>") and true, or "" and false for
// the aggregate feeds (host:all / host:summary), non-host rooms and names
// that do not parse. The request_* room walks use it so every instance they
// touch is the one authz.ParseRoom would name for the same room — the same
// parse join_room admitted the room with.
func instanceOfRoom(room string) (string, bool) {
	parsed, err := authz.ParseRoom(room)
	if err != nil || !parsed.IsHost() || parsed.IsHostAggregate() {
		return "", false
	}
	return parsed.Instance, true
}
