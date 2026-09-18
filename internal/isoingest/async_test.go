package isoingest

import "testing"

// joinAsync waits, at cleanup, for every goroutine this package fired and
// forgot (extraction, thumbnails). Register it LAST in a helper: t.Cleanup runs
// LIFO, so the join then runs BEFORE t.TempDir's RemoveAll and app.Cleanup pull
// the tree and the database out from under a goroutine that is still writing
// ("directory not empty" / "sql: database is closed" in CI).
func joinAsync(t *testing.T) {
	t.Helper()
	t.Cleanup(asyncWG.Wait)
}
