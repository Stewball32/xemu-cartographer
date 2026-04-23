package admin

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(func() {
		Group.GET("/stats", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, map[string]string{
				"status": "admin area",
			})
		})
	})
}
