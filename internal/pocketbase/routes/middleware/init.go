package middleware

import "github.com/pocketbase/pocketbase/core"

var globalInits []func(se *core.ServeEvent)

func registerGlobal(fn func(se *core.ServeEvent)) {
	globalInits = append(globalInits, fn)
}

// Init applies all global middleware. Called before groups and routes.
func Init(se *core.ServeEvent) {
	for _, fn := range globalInits {
		fn(se)
	}
}
