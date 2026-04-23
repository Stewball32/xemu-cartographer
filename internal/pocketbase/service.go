package pocketbase

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/pocketbase/resolvers"
)

// Service wraps core.App and implements pbiface.Service
// by delegating to existing resolver functions.
type Service struct {
	app core.App
}

// NewService creates a Service backed by the given PocketBase app.
func NewService(app core.App) *Service {
	return &Service{app: app}
}

// FindUserByDiscordID looks up a PocketBase user record by their Discord ID.
func (s *Service) FindUserByDiscordID(discordID string) (*core.Record, error) {
	return resolvers.FindUserByDiscordID(s.app, discordID)
}
