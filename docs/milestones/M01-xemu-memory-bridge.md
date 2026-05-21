# Milestone 1 — xemu memory bridge

Foundation. Gets the server able to read memory from any xemu-running Xbox game.

**Status:** Ported. [internal/xemu/](../internal/xemu/) (mem.go, qmp.go, instance.go) and [internal/scraper/](../internal/scraper/) (scraper.go, types.go, state.go) match the legacy implementation. Smoke-tested against a native xemu install via `GET /api/admin/xemu/probe?sock=<path>` ([internal/pocketbase/routes/xemu/probe.go](../internal/pocketbase/routes/xemu/probe.go)): PID discovery + QMP handshake + base HVA + low-GVA translation + `/proc/<pid>/mem` reads all confirmed working — XBE magic at GVA `0x00010000` reads back as `0x48454258` ("XBEH"), title ID round-trips out of the certificate. Empty registry returns `detect: unknown title ID 0x...` as expected.

Small extensions on top of the legacy port:

- `findPID` now matches a bare `xemu*` binary in addition to `AppRun`, so native installs work alongside the containerised AppImage path.
- `Instance.PID` field and `Mem.Base()` accessor surfaced for diagnostics (used by the probe route).
- [internal/pocketbase/routes/middleware/admin.go](../internal/pocketbase/routes/middleware/admin.go) `RequireAdmin()` admits PocketBase superusers in addition to `users.isAdmin=true` records — aligns the middleware with what CLAUDE.md already documented.

Native-xemu host gotcha to remember for M2 dev work: xemu is typically installed with `CAP_NET_ADMIN | CAP_NET_RAW` file caps for pcap netplay, which makes the process non-dumpable and flips `/proc/<pid>/*` ownership to `root:root` — `/proc/<pid>/mem` becomes unreadable to the same UID even with `kernel.yama.ptrace_scope=0`. Workarounds: `sudo setcap -r $(which xemu)` (drops netplay caps), or grant the server binary `CAP_SYS_PTRACE` (bypasses both Yama and the dumpable check). M3's containerised deployment runs the server rooted inside the container PID namespace and side-steps both.
