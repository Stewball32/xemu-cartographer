package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(instagramProvider)
}

func instagramProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("INSTAGRAM_CLIENT_ID")
	clientSecret := os.Getenv("INSTAGRAM_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "instagram"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "instagram",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
