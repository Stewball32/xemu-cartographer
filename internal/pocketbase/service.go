package pocketbase

import (
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/resolvers"
	"github.com/pocketbase/pocketbase/core"
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

// FindGamertagsForUser returns every gamertag record owned by the given user.
func (s *Service) FindGamertagsForUser(userID string) ([]*core.Record, error) {
	return resolvers.FindGamertagsForUser(s.app, userID)
}

// FindUserByGamertagString finds the owner of a gamertag matching the given
// exact string. Returns (nil, nil) when no row matches.
func (s *Service) FindUserByGamertagString(tag string) (*core.Record, error) {
	return resolvers.FindUserByGamertagString(s.app, tag)
}

// FindActiveRostersForUser returns roster rows where the acting user is
// currently rostered on any team, with team + gamertag relations expanded.
func (s *Service) FindActiveRostersForUser(userID string) ([]*core.Record, error) {
	return resolvers.FindActiveRostersForUser(s.app, userID)
}
