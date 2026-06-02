package hooks

import (
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	register(registerNotificationsFieldLockHook)
}

// registerNotificationsFieldLockHook restricts API-driven updates on the
// notifications collection to the `read` + `read_at` fields. The schema's
// PB rules already gate ownership (recipient or admin), but rules can't
// express "owner may write, but only to these specific fields" — that's a
// hook concern.
//
// Without this gate, an owner could PATCH their own notification rows to
// change `type` or `payload_json`, which would silently corrupt the bell UI
// (rendering against an unknown type / mis-shaped payload).
//
// Auto-stamps `read_at` to now() when `read` flips false → true so the
// frontend doesn't have to set both fields. Idempotent: subsequent writes
// that leave `read` at true don't re-stamp; writes that flip true → false
// clear `read_at` so the row goes back to genuinely unread state.
//
// Admin writes (via the SDK as a superuser, or as an isAdmin user) skip the
// field gate so back-office tooling can correct mis-delivered rows.
func registerNotificationsFieldLockHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdateRequest("notifications").BindFunc(func(e *core.RecordRequestEvent) error {
		actorIsAdmin := e.Auth != nil && (e.Auth.IsSuperuser() || e.Auth.GetBool("isAdmin"))

		prev := e.Record.Original()
		readChanged := prev.GetBool("read") != e.Record.GetBool("read")

		if !actorIsAdmin {
			// Non-admin updates may only touch `read` (+ derived `read_at`).
			// Anything else is a tampering attempt; reject the whole write.
			if prev.GetString("user") != e.Record.GetString("user") ||
				prev.GetString("type") != e.Record.GetString("type") ||
				prev.GetString("payload_json") != e.Record.GetString("payload_json") {
				return apis.NewBadRequestError("notifications only allow updating the read flag", nil)
			}
		}

		if readChanged {
			if e.Record.GetBool("read") {
				if t, err := types.ParseDateTime(time.Now().UTC()); err == nil {
					e.Record.Set("read_at", t)
				}
			} else {
				e.Record.Set("read_at", nil)
			}
		}

		return e.Next()
	})
}
