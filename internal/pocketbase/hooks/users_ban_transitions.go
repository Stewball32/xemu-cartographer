package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

func init() {
	register(registerUsersBanTransitionsHook)
}

// registerUsersBanTransitionsHook is the M8f write-side gate for the new
// is_banned + banned_until fields on users. Three responsibilities:
//
//  1. Reject non-admin PATCH attempts that try to mutate either field.
//     The AuthRule blocks banned users from authenticating; this hook
//     blocks a banned (or about-to-be-banned) user from clearing their
//     own flag through the default users update rule.
//
//  2. Emit the right audit_log action for each transition:
//     - is_banned false → true                       → ActionBan
//     - is_banned true  → false                      → ActionUnban
//     - banned_until changes while is_banned = true  → ActionTimeout
//     A single API call can produce multiple events (banning + setting a
//     timeout simultaneously fires ActionBan + ActionTimeout) so the
//     reader can disambiguate downstream.
//
//  3. Auto-stamp banned_until → empty when is_banned flips true → false,
//     so a stale future timestamp doesn't linger and confuse the next ban.
//
// The reason payload stays empty on direct SDK writes — the M8g admin
// route handlers will set it explicitly via audit.Write before the SDK
// patch hits the record. Empty reason audit rows still encode the actor +
// timestamp, which is the load-bearing signal.
func registerUsersBanTransitionsHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdateRequest("users").BindFunc(func(e *core.RecordRequestEvent) error {
		prev := e.Record.Original()

		prevBanned := prev.GetBool("is_banned")
		nextBanned := e.Record.GetBool("is_banned")
		prevUntil := prev.GetString("banned_until")
		nextUntil := e.Record.GetString("banned_until")

		bannedChanged := prevBanned != nextBanned
		untilChanged := prevUntil != nextUntil

		if !bannedChanged && !untilChanged {
			return e.Next()
		}

		if !roles.IsAdminAuth(e.App, e.Auth) {
			return apis.NewForbiddenError("only admins may set ban or timeout state", nil)
		}

		// If banning flips true → false, clear the residual timeout so a
		// stale future date doesn't make the next ban look already-expired.
		if bannedChanged && prevBanned && !nextBanned {
			e.Record.Set("banned_until", "")
			nextUntil = ""
			untilChanged = prevUntil != ""
		}

		// ActionBan: false → true transition.
		if bannedChanged && !prevBanned && nextBanned {
			if err := audit.Write(e.App, e.Auth, audit.ActionBan, e.Record, audit.BanPayload{}); err != nil {
				e.App.Logger().Error("M8f: audit ActionBan failed", "user", e.Record.Id, "err", err)
			}
		}

		// ActionUnban: true → false transition.
		if bannedChanged && prevBanned && !nextBanned {
			if err := audit.Write(e.App, e.Auth, audit.ActionUnban, e.Record, audit.UnbanPayload{}); err != nil {
				e.App.Logger().Error("M8f: audit ActionUnban failed", "user", e.Record.Id, "err", err)
			}
		}

		// ActionTimeout: banned_until changed while is_banned is (or
		// becomes) true. Captures both "ban with a timer" (false → true
		// AND banned_until set) and "extend an existing ban" (banned_until
		// changed while is_banned stays true).
		if untilChanged && nextBanned && nextUntil != "" {
			if err := audit.Write(e.App, e.Auth, audit.ActionTimeout, e.Record, audit.TimeoutPayload{
				ExpiresAt: nextUntil,
			}); err != nil {
				e.App.Logger().Error("M8f: audit ActionTimeout failed", "user", e.Record.Id, "err", err)
			}
		}

		return e.Next()
	})
}
