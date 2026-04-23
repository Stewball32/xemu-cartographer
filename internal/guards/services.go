package guards

import (
	"github.com/pocketbase/pocketbase/core"
	discordiface "github.com/youruser/yourproject/internal/guards/interfaces/discord"
	pbiface "github.com/youruser/yourproject/internal/guards/interfaces/pocketbase"
	wsiface "github.com/youruser/yourproject/internal/guards/interfaces/websocket"
)

// Services bundles all system access a guard or resolver may need.
// Fields may be nil if the corresponding system is not running.
type Services struct {
	App     core.App
	Discord discordiface.Service
	WS      wsiface.Service
	PB      pbiface.Service
}
