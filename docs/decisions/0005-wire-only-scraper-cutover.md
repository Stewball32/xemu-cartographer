# ADR-0005 — Wire-only scraper: the league consumes the xc-scraper daemon over its frozen feed

> **Status:** Accepted
> **Date:** 2026-09-07

## Context

Until step 7 the memory scraper was a set of goroutines inside the league binary:
`cmd/server` built the runner, discovery attached it to every QMP socket, and the
league glue (`internal/leaguescraper`) fanned its envelopes into WebSocket rooms and
persisted finished games in-process. Step 7 moved the runner, the game plugins and the
wire contract into the sibling module `github.com/xemu-cartographer/xc-scraper`
(consumed through `replace => ../xc-scraper`), and M31 froze the v1 wire dialect. The
runner was still linked into the league, so every reader change was a league release,
a scraper panic took PocketBase down with it, and nothing but the league could read
the feed.

The step-8 design (`DESIGN-STEP8.md`, decisions D-1 … D-16) had to settle four things:
where the process boundary goes, whether the wire changes, how the switch is made
without a flag day, and where the player-hosting runner (ADR-0003's programmatic
input channel) lives once the scraper is out of process. Constraints: the vendored TS
types and every browser page must keep working unchanged; the podman stack runs xemu
as root and the scraper reads `/proc/<pid>/mem`, so the daemon has to run natively on
the same host; the operator is one person with one LAN night a month to soak a change.

## Decision

1. **Topology (D-1).** `xc-scraper` becomes a daemon (`cmd/xc-scraper`) that attaches
   to the instances and serves the wire feed on its own WebSocket (`/api/ws`). The
   league **dials it as a consumer**: hot reads (state classes, summary, hello data)
   come from a league-side **mirror** fed by the stream; cold reads and RPCs
   (`request_events`, `request_probe`, inspect, maps, host control, xemu diagnostics)
   go over the daemon's HTTP read/control API; finished games arrive by **webhook**
   (`POST /api/xc/finished_game`, spool-first on the daemon side, D-6).
2. **Zero wire-dialect changes (D-2).** No new `Message` fields, classes or hello
   fields; the golden fixtures stay byte-identical; the league **rebroadcasts the
   daemon's frames byte-for-byte** into the same `host:*` rooms. The only extension is
   the `xc:` room family (`xc:hostrunner`), which the league never forwards to
   browsers. RPC replies are never shared on the rebroadcast stream — the league
   forwards `request_*` over daemon HTTP so a reply cannot reach the wrong client.
3. **Cut-over in two releases (D-4).** R1 (this milestone): `XC_SCRAPER_URL` set ⇒
   wire mode — the runner is never constructed, discovery never starts; unset ⇒ the
   in-process path exactly as before. R2 (after pre and prod have soaked on wire mode)
   deletes the in-process path; unset then means no feed. Roll-back in R1 is a
   per-process flag plus the rule "stop the daemon **before** the league re-attaches".
4. **Host runner in the daemon (D-9).** hostrunner / vncinput / customvariants run
   inside the daemon behind `--hostrunner` (default off), driven through the control
   API and observed on `xc:hostrunner`. ADR-0003's channel (programmatic xemu input
   over the instance's VNC) is unchanged; only the process that speaks it moves.
5. **One daemon per tier, natively (D-14, §11).** Ports 8990 prod / 8991 pre / 8992
   dev, a root systemd unit per tier sharing the tier's `.env`, `bin/xc-scraper` built
   by the same `task build` that builds `bin/server`. Feed token and control token are
   separate secrets (D-3).

## Consequences

- **Positive.** The feed survives league restarts (the daemon spools finished games
  until the webhook answers 2xx) and the league survives scraper crashes (it
  reconnects forever; the mirror clears after `XC_SCRAPER_STALE_AFTER`). Any consumer
  that speaks the frozen v1 dialect — overlays, OBS sources, a second league, a
  conformance test — can read the daemon directly. Browsers, the vendored TS mirror
  and the WS handler contract are untouched, so the cut-over is invisible to the
  frontend. The dev loop stays one command (`task dev` runs the daemon, backend and
  frontend under Air/Vite).
- **Negative.** Two processes per tier means two units, two boot lines, two health
  checks and one more secret to keep in sync; the owner must claim the port block and
  install the units. R1 keeps **both** feed paths in the tree, so every scraper-facing
  change is written twice until R2 (`bootScraperFeed` is the single fork point to keep
  that honest). Running the league in-process **and** a daemon on the same QMP
  directory duplicates every finished game — the RUNBOOK's rollback order is a rule,
  not a hint. Some request/reply latency moves from an in-memory call to loopback HTTP
  (inspect state is cached 250 ms with single-flight, D-16).
- **Neutral.** The league keeps a types-only import of `xc-scraper/runner` after R2
  (D-15); the R2 gate is "no `daemon`/`hub`/`discovery` in `cmd/server`'s dependency
  closure and no `runner.New` / `discovery.NewWatcher` outside tests", not "no
  runner import". Roster filtering stays daemon-side from the pushed control document
  (D-8); the league pushes dummy tags, neutral hosts and offset sets rather than
  filtering itself. `sudo task dev` stays in step 8 (D-13); the privilege split is an
  R2 follow-up.
