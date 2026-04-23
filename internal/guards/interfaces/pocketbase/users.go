package pocketbase

import "github.com/pocketbase/pocketbase/core"

// Users abstracts PocketBase user lookups.
type Users interface {
	FindUserByDiscordID(discordID string) (*core.Record, error)
}
