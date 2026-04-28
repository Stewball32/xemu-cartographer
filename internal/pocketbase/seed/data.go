//go:build dev

package seed

import "os"

type seedSuperuser struct{ Email, Password string }

var superusers []seedSuperuser

type seedUser struct {
	Email, Password, Username string
	IsAdmin                   bool
}

var users = []seedUser{
	{Email: "admin@dev.com", Password: "admin123", Username: "admin", IsAdmin: true},
}

func init() {
	email := os.Getenv("PB_SUPERUSER_EMAIL")
	password := os.Getenv("PB_SUPERUSER_PASSWORD")
	if email != "" && password != "" {
		superusers = append(superusers, seedSuperuser{Email: email, Password: password})
	}
}
