package oauth

import (
	"os"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(githubProvider)
}

func githubProvider() (core.OAuth2ProviderConfig, bool) {
	clientId := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return core.OAuth2ProviderConfig{Name: "github"}, false
	}

	return core.OAuth2ProviderConfig{
		Name:         "github",
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}, true
}
