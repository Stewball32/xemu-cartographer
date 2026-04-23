package resolvers

import (
	"errors"

	"github.com/disgoorg/snowflake/v2"
	"github.com/youruser/yourproject/internal/guards"
)

// GetGuildMember checks whether a Discord user is a member of a guild.
func GetGuildMember(svc *guards.Services, guildID, userID snowflake.ID) (bool, error) {
	if svc.Discord == nil {
		return false, errors.New("discord service not available")
	}
	return svc.Discord.IsMember(guildID, userID)
}

// GetUserRoles returns the Discord role IDs for a user in a guild.
func GetUserRoles(svc *guards.Services, guildID, userID snowflake.ID) ([]snowflake.ID, error) {
	if svc.Discord == nil {
		return nil, errors.New("discord service not available")
	}
	return svc.Discord.MemberRoles(guildID, userID)
}
