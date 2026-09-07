# WebSocket API — moved

The WebSocket wire contract (frames, envelope, rooms, classes, payload shapes, tiers,
`finished_game`, compatibility policy) now lives with its implementation in the
**xc-scraper** repository: [`xc-scraper/docs/wire.md`](../../xc-scraper/docs/wire.md)
(sibling checkout `../xc-scraper`; module `github.com/xemu-cartographer/xc-scraper`).

The Go types are `github.com/xemu-cartographer/xc-scraper/wire` (aliased here by `internal/websocket`,
`internal/websocket/rooms` and `internal/scraper/manager`); the TypeScript mirror is vendored into
`sveltekit/src/lib/types/scraper-v2.ts` by `task sync-wire`. Host policy (auth doors, `?console=`, room gating) is its §1.
