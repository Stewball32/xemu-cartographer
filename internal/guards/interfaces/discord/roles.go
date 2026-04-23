package discord

import "github.com/disgoorg/snowflake/v2"

// Roles abstracts Discord role lookups.
type Roles interface {
	MemberRoles(guildID, userID snowflake.ID) ([]snowflake.ID, error)
}
