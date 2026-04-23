//go:build dev

package seed

import "os"

type seedSuperuser struct{ Email, Password string }
type seedUser struct{ Email, Password, Name, Username string }
type seedPost struct{ Title, Body, AuthorEmail string }

var superusers []seedSuperuser

func init() {
	email := os.Getenv("PB_SUPERUSER_EMAIL")
	password := os.Getenv("PB_SUPERUSER_PASSWORD")
	if email != "" && password != "" {
		superusers = append(superusers, seedSuperuser{Email: email, Password: password})
	}
}

var users = []seedUser{
	{Email: "alice@dev.com", Password: "password1234", Name: "Alice", Username: "alice"},
	{Email: "bob@dev.com", Password: "password1234", Name: "Bob", Username: "bob"},
}

var posts = []seedPost{
	{Title: "Hello World", Body: "First test post.", AuthorEmail: "alice@dev.com"},
}
