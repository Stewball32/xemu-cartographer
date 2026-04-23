package guards

import (
	"errors"

	"github.com/disgoorg/snowflake/v2"
	"github.com/pocketbase/pocketbase/core"
)

// RequireDiscordRole checks that the user has a specific Discord role in a guild.
func RequireDiscordRole(guildID, roleID snowflake.ID) GuardFunc {
	return func(svc *Services, user *core.Record) error {
		if user == nil {
			return ErrAuthRequired
		}
		if svc.Discord == nil {
			return errors.New("discord service not available")
		}
		discordID := user.GetString("discordId")
		if discordID == "" {
			return errors.New("no linked Discord account")
		}
		uid, err := snowflake.Parse(discordID)
		if err != nil {
			return errors.New("invalid Discord ID on user record")
		}
		roles, err := svc.Discord.MemberRoles(guildID, uid)
		if err != nil {
			return err
		}
		for _, r := range roles {
			if r == roleID {
				return nil
			}
		}
		return ErrForbidden
	}
}
