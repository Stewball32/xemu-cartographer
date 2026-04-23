package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(microsoftProvider)
}

func microsoftProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("MICROSOFT_CLIENT_ID")
	clientSecret := os.Getenv("MICROSOFT_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "microsoft"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "microsoft",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
