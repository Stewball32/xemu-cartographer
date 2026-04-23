package routes

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerMeRoute)
}

func registerMeRoute(se *core.ServeEvent) {
	se.Router.GET("/api/me", func(e *core.RequestEvent) error {
		return e.JSON(http.StatusOK, map[string]any{
			"id":    e.Auth.Id,
			"email": e.Auth.Email(),
		})
	}).Bind(apis.RequireAuth())
}
