package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerPostsCollection)
}

func registerPostsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "posts") {
		return nil
	}

	users, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("posts")

	collection.Fields.Add(
		&core.TextField{
			Name:     "title",
			Required: true,
			Min:      1,
			Max:      200,
		},
		&core.EditorField{
			Name: "body",
		},
		&core.RelationField{
			Name:         "author",
			CollectionId: users.Id,
			Required:     true,
			MaxSelect:    1,
		},
	)

	// API rules: empty string = any authenticated user, nil = no API access.
	collection.ListRule = strPtr("")
	collection.ViewRule = strPtr("")
	collection.CreateRule = strPtr("")
	collection.UpdateRule = strPtr("author = @request.auth.id")
	collection.DeleteRule = strPtr("author = @request.auth.id")

	return app.Save(collection)
}
