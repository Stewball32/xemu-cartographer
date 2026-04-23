package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(patreonProvider)
}

func patreonProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("PATREON_CLIENT_ID")
	clientSecret := os.Getenv("PATREON_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "patreon"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "patreon",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
