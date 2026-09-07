# WebSocket API — moved

The WebSocket wire contract (frames, envelope, rooms, classes, payload shapes, tiers,
`finished_game`, compatibility policy) now lives with its implementation in the
**xc-scraper** repository: [`xc-scraper/docs/wire.md`](../../xc-scraper/docs/wire.md)
(sibling checkout `../xc-scraper`; module `github.com/xemu-cartographer/xc-scraper`).

The Go types are `github.com/xemu-cartographer/xc-scraper/wire` (aliased here by `internal/websocket` and
`internal/websocket/rooms`; the runner that emits them is `xc-scraper/runner`, framed into rooms by
`internal/leaguescraper`); the TypeScript mirror is vendored into `sveltekit/src/lib/types/scraper-v2.ts`
and the golden fixtures into `sveltekit/src/lib/types/wire-fixtures/` by `task sync-wire`
(`task sync-wire:check` / the CI "Wire sync check" step fail on drift). Host policy (auth doors, `?console=`, room gating) is its §1.
