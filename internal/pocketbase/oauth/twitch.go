package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(twitchProvider)
}

func twitchProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("TWITCH_CLIENT_ID")
	clientSecret := os.Getenv("TWITCH_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "twitch"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "twitch",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
