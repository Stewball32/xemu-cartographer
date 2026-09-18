package migrations

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// Authz step 1 (design §5.1): roles carry a `scopes` JSON list that the authz
// engine matches against (`authz.Match`). The seed rows come from
// authz.SeedRoles — the single source of truth the dev seeder also reads — so
// every deployment has the five baseline roles with their default scopes.
//
// Existing rows are preserved: `scopes` is only written when the field is
// empty/null (operator edits win), `level` only when the row still has the
// pre-M08 zero and the seed gives a real one. Down removes the field and
// keeps every row.
func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("roles")
		if err != nil {
			return err
		}
		if c.Fields.GetByName("scopes") == nil {
			c.Fields.Add(&core.JSONField{Name: "scopes", MaxSize: 65536})
			if err := app.Save(c); err != nil {
				return err
			}
		}

		for _, seed := range authz.SeedRoles {
			scopes := append([]string{}, seed.Scopes...) // never nil → json "[]"
			rec, err := app.FindFirstRecordByData(c, "slug", seed.Slug)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if rec == nil {
				rec = core.NewRecord(c)
				rec.Set("slug", seed.Slug)
				rec.Set("label", seed.Label)
				rec.Set("level", seed.Level)
				rec.Set("scopes", scopes)
				if err := app.Save(rec); err != nil {
					return err
				}
				continue
			}
			changed := false
			if jsonEmpty(rec.GetString("scopes")) {
				rec.Set("scopes", scopes)
				changed = true
			}
			if rec.GetInt("level") == 0 && seed.Level != 0 {
				rec.Set("level", seed.Level)
				changed = true
			}
			if changed {
				if err := app.Save(rec); err != nil {
					return err
				}
			}
		}
		return nil
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("roles")
		if err != nil {
			return err
		}
		if c.Fields.GetByName("scopes") == nil {
			return nil
		}
		c.Fields.RemoveByName("scopes")
		return app.Save(c)
	})
}

// jsonEmpty mirrors PocketBase's empty-JSON test for a JSONField's raw value
// (null / "" / [] / {} / no value).
func jsonEmpty(raw string) bool {
	switch strings.TrimSpace(raw) {
	case "", "null", `""`, "[]", "{}":
		return true
	}
	return false
}
