package routes

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
	"github.com/xemu-cartographer/xemu-cartographer/internal/version"
)

func init() {
	register(registerVersionRoute)
}

func registerVersionRoute(se *core.ServeEvent) {
	se.Router.GET("/api/version", func(e *core.RequestEvent) error {
		return e.JSON(http.StatusOK, map[string]string{
			"version": version.Version,
			"commit":  version.Commit,
			"date":    version.Date,
		})
	})
}
