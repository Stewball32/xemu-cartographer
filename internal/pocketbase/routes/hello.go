package routes

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerHelloRoute)
}

func registerHelloRoute(se *core.ServeEvent) {
	se.Router.GET("/api/hello", func(e *core.RequestEvent) error {
		return e.JSON(http.StatusOK, map[string]string{
			"message": "Hello from the template!",
		})
	})
}
