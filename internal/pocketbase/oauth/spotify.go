package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(spotifyProvider)
}

func spotifyProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "spotify"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "spotify",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
