//go:build dev

package seed

import "os"

type seedSuperuser struct{ Email, Password string }

var superusers []seedSuperuser

func init() {
	email := os.Getenv("PB_SUPERUSER_EMAIL")
	password := os.Getenv("PB_SUPERUSER_PASSWORD")
	if email != "" && password != "" {
		superusers = append(superusers, seedSuperuser{Email: email, Password: password})
	}
}
