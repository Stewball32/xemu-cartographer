package routes

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/pocketbase/routes/admin"
)

// registerAllGroups creates all route groups and their routes.
// To add a new group: import the group package and call its RegisterAll here.
func registerAllGroups(se *core.ServeEvent) {
	admin.RegisterAll(se)
}
