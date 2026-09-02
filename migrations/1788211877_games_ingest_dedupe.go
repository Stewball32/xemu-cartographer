package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// League-side ingest hardening (pre-freeze fix 8): `game_uid` is the
// scraper-minted idempotency key for at-least-once finished-game delivery —
// unique so games.PersistFinishedGame can treat a duplicate insert as a
// redelivery and no-op instead of double-applying the Elo chain. The index is
// partial (WHERE game_uid != '') because legacy rows and legacy callers carry
// no uid and must not collide with each other. `end_reason` records the
// scraper's observed match-exit cause ("postgame" / "left_match" /
// "shutdown"), plain text on purpose — the enum is an open set owned by the
// scraper contract.
func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("games")
		if err != nil {
			return err
		}
		if c.Fields.GetByName("game_uid") == nil {
			c.Fields.Add(&core.TextField{Name: "game_uid", Max: 64})
		}
		if c.Fields.GetByName("end_reason") == nil {
			c.Fields.Add(&core.TextField{Name: "end_reason", Max: 32})
		}
		c.AddIndex("idx_games_game_uid_unique", true, "game_uid", "game_uid != ''")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("games")
		if err != nil {
			return err
		}
		c.RemoveIndex("idx_games_game_uid_unique")
		c.Fields.RemoveByName("game_uid")
		c.Fields.RemoveByName("end_reason")
		return app.Save(c)
	})
}
