package routes

import (
	"os"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerAppRoute)
}

func registerAppRoute(se *core.ServeEvent) {
	se.Router.GET("/app/{path...}", func(e *core.RequestEvent) error {
		return e.FileFS(os.DirFS("pb_public"), "index.html")
	}).Bind(apis.RequireAuth())
}
