package guards

import (
	"errors"

	"github.com/disgoorg/snowflake/v2"
	"github.com/pocketbase/pocketbase/core"
)

// RequireGuildMember checks that the user's linked Discord account is a member
// of the specified guild. Looks up the discordId field on the PB record.
func RequireGuildMember(guildID snowflake.ID) GuardFunc {
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
		ok, err := svc.Discord.IsMember(guildID, uid)
		if err != nil {
			return err
		}
		if !ok {
			return ErrForbidden
		}
		return nil
	}
}
