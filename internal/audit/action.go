// Package audit owns the write path for the audit_log collection defined
// in ADR-0002. Callers (typically PB record hooks) invoke Write or WriteRef
// rather than constructing audit_log records directly, so the payload-JSON
// shape stays consistent across every collection that emits audit events.
//
// # Action + payload convention
//
// Each distinct action lives in its own file named action_<verb>.go. The
// file declares two things:
//
//  1. An Action constant: `const ActionBlock Action = "block"`.
//  2. A matching payload struct: `type BlockPayload struct{ Reason string `+"`"+`json:"reason"`+"`"+` }`.
//
// Callers always pair them: `audit.Write(app, actor, audit.ActionBlock, target, audit.BlockPayload{Reason: "spam"})`.
// The one-file-per-action layout matches the repo's hooks/, schema/, and
// guards/ conventions; adding a new action in M22b / M8 / M13 is a new
// file, not an edit to a central registry.
//
// Substage 22a ships only the Action type + this convention. The first
// real action files (block / unblock / approve) land with 22b alongside
// the gamertag 4-state moderation work.
package audit

// Action is the string-valued discriminator stored in the audit_log.action
// column. Defined as a named type so the helper signatures (Write / WriteRef)
// can't accept arbitrary strings — callers must pass an exported Action
// constant declared in one of the action_*.go files.
type Action string
