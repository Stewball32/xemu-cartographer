package guards

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	discordiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/discord"
	pbiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/pocketbase"
	scraperiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/scraper"
	wsiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/websocket"
)

// Services bundles all system access a guard or resolver may need.
// Fields may be nil if the corresponding system is not running.
type Services struct {
	App     core.App
	Discord discordiface.Service
	WS      wsiface.Service
	PB      pbiface.Service
	Scraper scraperiface.Service

	// Authz is the process-wide authorization adapter main.go installs in
	// OnServe (the same pointer pb.SetDefault publishes). Guards and route
	// groups keep reading pb.Default() at request time; this field exists
	// so subsystems holding *Services can reach the adapter without the
	// global. nil until boot installs it — authz.Can denies on nil deps.
	Authz *pb.PBDeps
}
