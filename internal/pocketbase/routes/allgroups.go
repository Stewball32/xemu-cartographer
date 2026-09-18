package routes

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/admin"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/adminusers"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/containers"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/isos"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/lansaves"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/lansync"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/play"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/pod"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/rosters"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/team_membership_requests"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/teams"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/tokens"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/xc"
	"github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/xemu"
)

// registerAllGroups creates all route groups and their routes.
// To add a new group: import the group package and call its RegisterAll here.
func registerAllGroups(se *core.ServeEvent) {
	admin.RegisterAll(se)
	adminusers.RegisterAll(se)
	containers.RegisterAll(se)
	isos.RegisterAll(se)
	lansaves.RegisterAll(se)
	lansync.RegisterAll(se)
	play.RegisterAll(se)
	pod.RegisterAll(se)
	rosters.RegisterAll(se)
	scraper.RegisterAll(se)
	team_membership_requests.RegisterAll(se)
	teams.RegisterAll(se)
	tokens.RegisterAll(se)
	xc.RegisterAll(se)
	xemu.RegisterAll(se)
}
