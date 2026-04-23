package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerPostsHooks)
}

func registerPostsHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreate("posts").BindFunc(func(e *core.RecordEvent) error {
		log.Printf("Post created: %s by %s", e.Record.GetString("title"), e.Record.GetString("author"))

		// For async work (e.g., sending Discord notifications), clone data
		// into local variables before the goroutine — event objects are not
		// concurrent-safe:
		//
		// title := e.Record.GetString("title")
		// authorId := e.Record.GetString("author")
		// routine.FireAndForget(func() {
		//     // Send Discord notification, update external systems, etc.
		//     log.Printf("Async: notifying about post %q by %s", title, authorId)
		// })

		return e.Next()
	})
}
