// Package authz is the pure authorization core (step 6 of the platform
// split): principal / resource / action types, the scope grammar, the rule
// table and Can, opaque-token helpers, WebSocket room helpers and the role
// seed literal.
//
// The package imports only the standard library — everything it needs from
// PocketBase, the scraper manager or the provisioner arrives through the Deps
// interface (deps.go), which internal/authz/pb implements against the live app
// and internal/authz/authztest fakes for tests. That boundary is what keeps
// Can unit-testable and lets every other slice (adapter, routes, hooks,
// WebSocket, boot) be written against this API.
//
// Evaluation is fail-closed everywhere: unknown actions, unknown principal
// kinds, nil deps, malformed scopes and malformed room names all deny.
// The design this package implements is xemu-cartographer-split/authz/
// DESIGN-STEP6.md (§2–§5); the reason strings on Decision are stable and
// log-safe so the adapter can map them to HTTP / WS error codes.
package authz
