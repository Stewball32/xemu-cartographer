package resolvers

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// FindUserByDiscordID looks up a PocketBase user record by their Discord ID.
// Returns nil and an error if no matching user is found.
func FindUserByDiscordID(app core.App, discordID string) (*core.Record, error) {
	record, err := app.FindFirstRecordByFilter(
		"users",
		"discordId = {:id}",
		dbx.Params{"id": discordID},
	)
	if err != nil || record == nil {
		return nil, fmt.Errorf("no PocketBase user found for Discord ID %s", discordID)
	}
	return record, nil
}
