// Death-event → per-player "killed by" projection.
//
// The POV overlay's respawn ring (CL-18) carries a KILLED BY plate naming the
// killer. That name isn't on the roster payload — it only exists in the event
// stream — so this module folds the two together.
//
// Pure and side-effect free so the newest-wins and name-matching rules are
// unit-testable; the page just calls withKilledBy on the mapped roster.

import type { AnyEvent, DeathEvent } from '$lib/types/scraper-v2';
import type { OverlayPlayer } from './overlay-split';

/** CE space-pads profile names and the roster mappers trim them, so both sides
 * of the match have to be trimmed or the lookup silently misses. */
const key = (s: string | undefined | null) => (s ?? '').trim();

/**
 * Map each victim's trimmed name → the trimmed name of whoever killed them,
 * or null when the death had no attributed killer (suicide / fall /
 * environment, or a killer the server withheld).
 *
 * Newest wins: the feed is newest-first, so the first entry seen for a victim
 * is their most recent death and later (older) ones are ignored. A death with
 * no killer still records an entry — it has to overwrite any stale killer from
 * an earlier death, otherwise a player who suicides right after being killed
 * would keep the old KILLED BY plate.
 */
export function killedByMap(events: AnyEvent[] | null | undefined): Record<string, string | null> {
	const out: Record<string, string | null> = {};
	if (!events) return out;
	for (const ev of events) {
		if (ev.event_type !== 'death') continue;
		const death = ev as DeathEvent;
		const victim = key(death.victim?.name);
		if (!victim || victim in out) continue;
		const killer = key(death.killer?.name);
		out[victim] = killer || null;
	}
	return out;
}

/**
 * Return a copy of `players` with `killedBy` filled in from the event feed.
 * Players with no death on record get null rather than being left undefined,
 * so the ring renders the RESPAWNING pill instead of a stale killer.
 */
export function withKilledBy(
	players: OverlayPlayer[],
	events: AnyEvent[] | null | undefined
): OverlayPlayer[] {
	const byVictim = killedByMap(events);
	return players.map((p) => ({ ...p, killedBy: byVictim[key(p.name)] ?? null }));
}
