// Client-side port of the Go M10d roster filter (internal/scraper/roster.
// FilterRoster). The WS-push console path subscribes to the UNFILTERED
// host:<instance> broadcast, so the neutral-host dummy must be dropped here —
// the HTTP console route pre-filters its snapshot server-side, but the WS
// envelopes are raw. Config comes from the resolver's `filter` field
// (is_neutral_host + the raw dummy_gamertags list). Client-side filtering is
// acceptable for the PoC ("secure later"); keep it byte-for-byte equivalent to
// the Go filter so both surfaces agree.

/** Mirrors the resolver's `filter` object (overlay_pov.go). */
export interface DummyFilterConfig {
	is_neutral_host: boolean;
	/** Raw (unsanitized) dummy gamertag strings — sanitized here to match. */
	dummy_gamertags: string[];
}

/** Mirrors roster.SanitizeName: lowercased + trimmed. */
function sanitizeName(s: string): string {
	return s.trim().toLowerCase();
}

/**
 * filterRoster drops a neutral host's local dummy player(s) and any globally
 * allowlisted dummy gamertag — the exact rules of the Go FilterRoster. Pure:
 * never mutates the input, returns it unchanged when cfg is null (no-op, so an
 * unconfigured / poll-path roster passes through). Generic over any player shape
 * carrying `name` + `is_local` (GameRosterPlayer).
 */
export function filterRoster<T extends { name: string; is_local?: boolean | null }>(
	players: T[],
	cfg: DummyFilterConfig | null | undefined
): T[] {
	if (!cfg) return players;
	const set = new Set((cfg.dummy_gamertags ?? []).map(sanitizeName).filter(Boolean));
	return players.filter((p) => {
		if (cfg.is_neutral_host && p.is_local === true) return false;
		if (set.has(sanitizeName(p.name))) return false;
		return true;
	});
}
