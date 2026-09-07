package games_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/games"
)

func countRecords(t *testing.T, app core.App, collection string) int {
	t.Helper()
	rows, err := app.FindAllRecords(collection)
	if err != nil {
		t.Fatalf("FindAllRecords %s: %v", collection, err)
	}
	return len(rows)
}

func TestPersistFinishedGame_DedupesOnGameUID(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	fg := sampleGame()
	fg.GameUID = "01DEDUPE0000000000000000UID"
	fg.EndReason = "postgame"

	first, err := games.PersistFinishedGame(app, fg)
	if err != nil {
		t.Fatalf("first persist: %v", err)
	}
	if first.Deduped {
		t.Error("first persist should not be deduped")
	}

	g, err := app.FindRecordById("games", first.GameID)
	if err != nil {
		t.Fatalf("find game: %v", err)
	}
	if got := g.GetString("game_uid"); got != fg.GameUID {
		t.Errorf("game_uid = %q, want %q", got, fg.GameUID)
	}
	if got := g.GetString("end_reason"); got != "postgame" {
		t.Errorf("end_reason = %q, want postgame", got)
	}

	// At-least-once redelivery: same uid again → logged no-op pointing at the
	// existing row, nothing double-applied.
	second, err := games.PersistFinishedGame(app, fg)
	if err != nil {
		t.Fatalf("second persist: %v", err)
	}
	if !second.Deduped {
		t.Error("second persist should be deduped")
	}
	if second.GameID != first.GameID || second.SeriesID != first.SeriesID {
		t.Errorf("dedupe ids = (%s, %s), want (%s, %s)",
			second.GameID, second.SeriesID, first.GameID, first.SeriesID)
	}

	if n := countRecords(t, app, "games"); n != 1 {
		t.Errorf("games rows = %d, want 1", n)
	}
	if n := countRecords(t, app, "series"); n != 1 {
		t.Errorf("series rows = %d, want 1", n)
	}
	if n := countRecords(t, app, "game_players"); n != 2 {
		t.Errorf("game_players rows = %d, want 2", n)
	}
	for _, tag := range []string{"Stewie", "BlueFox"} {
		r, err := app.FindFirstRecordByFilter("ratings", "gamertag = {:g}", dbx.Params{"g": tag})
		if err != nil {
			t.Fatalf("find rating %s: %v", tag, err)
		}
		if r.GetInt("games") != 1 {
			t.Errorf("rating games for %s = %d, want 1 (applied once)", tag, r.GetInt("games"))
		}
	}
}

// TestGamesGameUIDUniqueIndexEnforced proves the partial unique index is a
// real DB constraint (the race-safety backstop), not just a pre-check.
func TestGamesGameUIDUniqueIndexEnforced(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	res, err := games.PersistFinishedGame(app, sampleGame())
	if err != nil {
		t.Fatalf("seed persist: %v", err)
	}

	col, err := app.FindCollectionByNameOrId("games")
	if err != nil {
		t.Fatalf("lookup games: %v", err)
	}
	mk := func(uid string) error {
		r := core.NewRecord(col)
		r.Set("series", res.SeriesID)
		r.Set("game_uid", uid)
		return app.Save(r)
	}
	if err := mk("01UNIQUE0000000000000000UID"); err != nil {
		t.Fatalf("first uid row: %v", err)
	}
	if err := mk("01UNIQUE0000000000000000UID"); err == nil {
		t.Error("duplicate game_uid insert should violate the unique index")
	}
	if err := mk(""); err != nil {
		t.Errorf("empty uid row alongside the seed's empty uid should save (partial index): %v", err)
	}
}

func TestPersistFinishedGame_EmptyGameUIDDoesNotCollide(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	for i := 0; i < 2; i++ {
		res, err := games.PersistFinishedGame(app, sampleGame())
		if err != nil {
			t.Fatalf("persist %d: %v", i+1, err)
		}
		if res.Deduped {
			t.Errorf("persist %d: empty GameUID must never dedupe", i+1)
		}
	}
	if n := countRecords(t, app, "games"); n != 2 {
		t.Errorf("games rows = %d, want 2 (empty uids don't collide)", n)
	}
}

func TestPersistFinishedGame_RollsBackOnFailure(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	// Fail the chain's LAST step (updateRatings) once, so every earlier write
	// — series, game, game_players, event stamping — must roll back.
	failOnce := true
	app.OnRecordCreate("ratings").BindFunc(func(e *core.RecordEvent) error {
		if failOnce {
			failOnce = false
			return errors.New("induced ratings failure")
		}
		return e.Next()
	})

	ev := insertEvent(t, app, "pod-a", winMid, "")

	fg := sampleGame()
	fg.GameUID = "01ROLLBACK00000000000000UID"

	if _, err := games.PersistFinishedGame(app, fg); err == nil {
		t.Fatal("expected the induced failure to surface")
	}
	for _, col := range []string{"games", "series", "game_players", "ratings"} {
		if n := countRecords(t, app, col); n != 0 {
			t.Errorf("%s rows after rollback = %d, want 0", col, n)
		}
	}
	if g := reload(t, app, ev.Id).GetString("game"); g != "" {
		t.Errorf("event stamp survived rollback, game=%q", g)
	}

	// The retry (the whole point of the rollback) now lands the full chain.
	res, err := games.PersistFinishedGame(app, fg)
	if err != nil {
		t.Fatalf("retry persist: %v", err)
	}
	if res.Deduped {
		t.Error("retry after rollback should persist, not dedupe")
	}
	if res.RatingsUpdated != 2 || res.EventsStamped != 1 {
		t.Errorf("retry chain = ratings %d / stamped %d, want 2 / 1",
			res.RatingsUpdated, res.EventsStamped)
	}
	if g := reload(t, app, ev.Id).GetString("game"); g != res.GameID {
		t.Error("retry did not stamp the in-window event")
	}

	// And a redelivery after the successful retry dedupes.
	res2, err := games.PersistFinishedGame(app, fg)
	if err != nil {
		t.Fatalf("redelivery persist: %v", err)
	}
	if !res2.Deduped {
		t.Error("redelivery after successful retry should dedupe")
	}
	if n := countRecords(t, app, "games"); n != 1 {
		t.Errorf("games rows = %d, want 1", n)
	}
}

// TestPersistFinishedGame_ConcurrentSameUID is the black-box outcome check:
// 8 same-uid deliveries → one row, one Elo application, no errors. PocketBase
// serialises writers on a single connection, so in practice the losers here
// are usually caught by the pre-check after the winner commits; the index
// backstop itself is forced deterministically by the next test.
func TestPersistFinishedGame_ConcurrentSameUID(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	fg := sampleGame()
	fg.GameUID = "01CONCURRENT000000000000UID"

	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = games.PersistFinishedGame(app, fg)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("delivery %d: %v", i, err)
		}
	}
	if got := countRecords(t, app, "games"); got != 1 {
		t.Errorf("games rows = %d, want 1", got)
	}
	for _, tag := range []string{"Stewie", "BlueFox"} {
		r, err := app.FindFirstRecordByFilter("ratings", "gamertag = {:g}", dbx.Params{"g": tag})
		if err != nil {
			t.Fatalf("find rating %s: %v", tag, err)
		}
		if r.GetInt("games") != 1 {
			t.Errorf("rating games for %s = %d, want 1", tag, r.GetInt("games"))
		}
	}
}

// TestPersistFinishedGame_IndexBackstopOnConcurrentInsert forces the path the
// race above only reaches by timing luck: a delivery whose pre-check ran
// before a rival's row committed. A "winner" parks inside an open transaction
// holding an uncommitted row with the uid (and, with it, PocketBase's single
// write connection); the "loser" then starts — its pre-check on the read pool
// can't see the row, so it proceeds to RunInTransaction and queues for the
// write connection (visible in the pool's wait counter). Releasing the winner
// lets the loser in, where its insert must trip idx_games_game_uid_unique,
// roll back its own series row, and come back Deduped with the winner's id.
func TestPersistFinishedGame_IndexBackstopOnConcurrentInsert(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	fg := sampleGame()
	fg.GameUID = "01BACKSTOP00000000000000UID"

	// Proof the loser really attempted (and lost) the insert, rather than
	// short-circuiting on the pre-check.
	var failedInserts atomic.Int32
	app.OnRecordAfterCreateError("games").BindFunc(func(e *core.RecordErrorEvent) error {
		failedInserts.Add(1)
		return e.Next()
	})

	seriesCol, err := app.FindCollectionByNameOrId("series")
	if err != nil {
		t.Fatalf("lookup series: %v", err)
	}
	winnerSeries := core.NewRecord(seriesCol)
	winnerSeries.Set("format", "exact-n")
	winnerSeries.Set("target_n", 1)
	winnerSeries.Set("category", "casual")
	if err := app.Save(winnerSeries); err != nil {
		t.Fatalf("seed winner series: %v", err)
	}

	inserted := make(chan string)
	release := make(chan struct{})
	winnerErr := make(chan error, 1)
	go func() {
		winnerErr <- app.RunInTransaction(func(tx core.App) error {
			col, err := tx.FindCollectionByNameOrId("games")
			if err != nil {
				return err
			}
			r := core.NewRecord(col)
			r.Set("series", winnerSeries.Id)
			r.Set("game_uid", fg.GameUID)
			if err := tx.Save(r); err != nil {
				return err
			}
			inserted <- r.Id
			<-release
			return nil
		})
	}()
	winnerID := <-inserted

	sqlDB := app.NonconcurrentDB().(*dbx.DB).DB()
	waitsBefore := sqlDB.Stats().WaitCount

	type outcome struct {
		res games.Result
		err error
	}
	loser := make(chan outcome, 1)
	go func() {
		res, err := games.PersistFinishedGame(app, fg)
		loser <- outcome{res, err}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for sqlDB.Stats().WaitCount == waitsBefore {
		if time.Now().After(deadline) {
			t.Fatal("loser never queued on the write connection (pre-check should have missed the uncommitted row)")
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err := <-winnerErr; err != nil {
		t.Fatalf("winner tx: %v", err)
	}

	got := <-loser
	if got.err != nil {
		t.Fatalf("loser: %v (the index backstop should have turned this into a dedupe)", got.err)
	}
	if !got.res.Deduped {
		t.Fatal("loser should report Deduped")
	}
	if got.res.GameID != winnerID || got.res.SeriesID != winnerSeries.Id {
		t.Errorf("loser ids = (%s, %s), want the winner's (%s, %s)",
			got.res.GameID, got.res.SeriesID, winnerID, winnerSeries.Id)
	}
	if n := failedInserts.Load(); n != 1 {
		t.Errorf("failed games inserts = %d, want 1 (the loser's insert must have hit the index)", n)
	}
	if n := countRecords(t, app, "games"); n != 1 {
		t.Errorf("games rows = %d, want 1", n)
	}
	// The loser auto-created a series inside its tx; the rollback must have
	// taken it with it — only the winner's seeded series survives.
	if n := countRecords(t, app, "series"); n != 1 {
		t.Errorf("series rows = %d, want 1 (loser's series should have rolled back)", n)
	}
	for _, col := range []string{"game_players", "ratings"} {
		if n := countRecords(t, app, col); n != 0 {
			t.Errorf("%s rows = %d, want 0 (nothing from the loser should persist)", col, n)
		}
	}
}
