package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(googleProvider)
}

func googleProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "google"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "google",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
