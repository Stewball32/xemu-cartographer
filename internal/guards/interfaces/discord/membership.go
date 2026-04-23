package discord

import "github.com/disgoorg/snowflake/v2"

// Membership abstracts guild membership lookups.
type Membership interface {
	IsMember(guildID, userID snowflake.ID) (bool, error)
}
