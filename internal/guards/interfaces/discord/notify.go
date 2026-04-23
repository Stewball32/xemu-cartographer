package discord

import "github.com/disgoorg/snowflake/v2"

// Notify abstracts sending messages to Discord channels.
type Notify interface {
	SendNotification(channelID snowflake.ID, content string) error
}
