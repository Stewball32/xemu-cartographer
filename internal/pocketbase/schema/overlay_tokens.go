package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerOverlayTokensCollection)
}

// registerOverlayTokensCollection creates the M10 overlay-token registry: one
// row per minted spectator/overlay token, so long-lived tokens are revocable +
// auditable. The signed JWT carries {room, scope, kid, exp}; this row is the
// kid's authority — the WS handshake verifies the signature then checks here
// that the kid exists, isn't revoked, isn't expired, and matches the room.
//
// Rules are nil (no direct REST CRUD): minting / revoking / listing all flow
// through the permission-gated /api/overlay-tokens routes (RequireAuth + the
// "manage overlays" check) which app.Save / FindRecords to bypass rules — the
// same pattern as the M08 roles/user_roles collections. Relates only to the
// built-in users collection, so no identity.go coordination is needed.
func registerOverlayTokensCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "overlay_tokens") {
		return nil
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("overlay_tokens")
	collection.Fields.Add(
		&core.TextField{Name: "kid", Required: true, Min: 1, Max: 64, Presentable: true},
		&core.TextField{Name: "room", Required: true, Min: 1, Max: 96, Presentable: true},
		&core.TextField{Name: "scope", Max: 32},
		&core.TextField{Name: "label", Max: 128},
		&core.RelationField{Name: "minted_by", CollectionId: usersCol.Id, MaxSelect: 1},
		&core.DateField{Name: "expires_at"},
		&core.BoolField{Name: "revoked"},
		&core.DateField{Name: "revoked_at"},
		&core.RelationField{Name: "revoked_by", CollectionId: usersCol.Id, MaxSelect: 1},
		&core.AutodateField{Name: "created", OnCreate: true},
	)

	collection.AddIndex("idx_overlay_tokens_kid_unique", true, "kid", "")
	collection.AddIndex("idx_overlay_tokens_room", false, "room", "")

	// nil rules — all access via the permission-gated custom routes.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
