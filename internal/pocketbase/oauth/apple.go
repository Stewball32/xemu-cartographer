package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(appleProvider)
}

func appleProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("APPLE_CLIENT_ID")
	clientSecret := os.Getenv("APPLE_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "apple"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "apple",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
