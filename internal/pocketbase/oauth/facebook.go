package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(facebookProvider)
}

func facebookProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("FACEBOOK_CLIENT_ID")
	clientSecret := os.Getenv("FACEBOOK_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "facebook"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "facebook",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
