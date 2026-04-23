package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(discordProvider)
}

func discordProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "discord"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "discord",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
