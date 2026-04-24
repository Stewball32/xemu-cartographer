package routes

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/admin"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/containers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/xemu"
)

// registerAllGroups creates all route groups and their routes.
// To add a new group: import the group package and call its RegisterAll here.
func registerAllGroups(se *core.ServeEvent) {
	admin.RegisterAll(se)
	containers.RegisterAll(se)
	xemu.RegisterAll(se)
}
