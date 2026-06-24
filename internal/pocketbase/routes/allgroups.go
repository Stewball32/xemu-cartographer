package routes

import (
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/admin"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/adminusers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/containers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/lansaves"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/overlays"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/pod"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/rosters"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/team_membership_requests"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/teams"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/xemu"
	"github.com/pocketbase/pocketbase/core"
)

// registerAllGroups creates all route groups and their routes.
// To add a new group: import the group package and call its RegisterAll here.
func registerAllGroups(se *core.ServeEvent) {
	admin.RegisterAll(se)
	adminusers.RegisterAll(se)
	containers.RegisterAll(se)
	lansaves.RegisterAll(se)
	overlays.RegisterAll(se)
	pod.RegisterAll(se)
	rosters.RegisterAll(se)
	scraper.RegisterAll(se)
	team_membership_requests.RegisterAll(se)
	teams.RegisterAll(se)
	xemu.RegisterAll(se)
}
