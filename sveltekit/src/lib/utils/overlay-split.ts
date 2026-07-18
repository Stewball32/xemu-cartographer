// Server-driven splitscreen detection + player mapping for the POV overlay.
//
// HOST-ONLY: cartographer scrapes ONLY the host box — never a player's machine.
// Everything here is derived from what the host scraper broadcasts over the WS,
// so the overlay reflects the HOST box's local splitscreen (which is where the
// split actually is). Nothing assumes per-player-box telemetry — that data does
// not exist.
//
// The exact host-reported wire fields consumed (verified against the real
// envelope in internal/scraper/manager/wire_split_test.go):
//   current_state (feed.game):
//     players[].is_local     — host-local splitscreen player (remotes = false)
//     players[].local_index  — the host viewport slot (0..3)
//     config.is_team_game    — team vs FFA (card tint)
//     players[].{name,team,armor_color,score,kills,deaths,assists,
//                kill_streak,shots_fired,shots_hit} — card stats
//   state_update (feed.tick):
//     locals[].local_index   — the host's per-local viewports (split fallback)
//     players[].{index,alive,health,shields,has_camo,respawn_in_ticks} — live state
//
// The split count is the number of HOST-LOCAL players; a system-link remote
// (is_local=false / local_index=null) is never a local and never counted. The
// engine's per-local viewport layout (internal/scraper/viewport.go) is
// 1→full, 2→top/bottom, 3-4→quads.
//
// Pure (no Svelte / no socket), so it's unit-tested against sample payloads and
// the overlay page just wires it to the reactive feed.

import type { GamePayload, TickPayloadV2 } from '$lib/types/scraper-v2';
import { CE_ARMOR_COLORS } from '$lib/data/halo-armor-palettes';
import { TICKS_PER_SECOND } from '$lib/utils/overlay-view';

/** The pack PlayerCard's expected shape (a subset of the OBS pack's contract). */
export interface OverlayPlayer {
	slot: number; // viewport index (local_index), top→bottom / left→right
	name: string;
	team: 'ffa' | 'red' | 'blue';
	armor: string; // hex tint for the FFA chip
	score: number;
	kills: number;
	deaths: number;
	assists: number;
	spree: number;
	accuracy: number; // 0..100
	damageRatio: number;
	alive: boolean;
	respawn: number; // seconds remaining (0 = alive / no countdown)
	respawnMax: number;
	camo: number; // seconds remaining (>0 cloaks the card)
	camoMax: number;
	shield: number; // 0..2 (>1 = overshield)
	health: number; // 0..1
}

const RESPAWN_MAX = 5;
const CAMO_MAX = 30;
const DEFAULT_ARMOR = '#9fb4d0';

/** Local (splitscreen) players from the roster, ordered by local_index. This is
 * the authoritative set the overlay lays out — its length IS the split count. */
function localRoster(game: GamePayload | null): GamePayload['players'] {
	const players = game?.players ?? [];
	return players
		.filter((p) => p.is_local === true && p.local_index != null)
		.slice()
		.sort((a, b) => (a.local_index ?? 0) - (b.local_index ?? 0));
}

/**
 * deriveSplitCount returns the live splitscreen configuration (number of local
 * players: 1 / 2 / 3 / 4), or 0 when there is no local game yet. Primary signal
 * is the roster's `is_local` players; falls back to the tick's `locals` array
 * (the engine's per-local viewports) if the roster hasn't populated is_local.
 * Clamped to the engine's 4-way max.
 */
export function deriveSplitCount(game: GamePayload | null, tick: TickPayloadV2 | null): number {
	const fromRoster = localRoster(game).length;
	const fromTick = tick?.locals?.length ?? 0;
	return Math.min(Math.max(fromRoster, fromTick), 4);
}

/** layoutKey clamps a split count into the pack's 1..4 layout keys (0 → 1, so an
 * idle overlay renders the single-view layout rather than breaking). */
export function layoutKey(splitCount: number): 1 | 2 | 3 | 4 {
	return Math.min(Math.max(splitCount, 1), 4) as 1 | 2 | 3 | 4;
}

function teamOf(isTeamGame: boolean, team: number): 'ffa' | 'red' | 'blue' {
	if (!isTeamGame) return 'ffa';
	return team === 1 ? 'blue' : 'red';
}

function armorHex(index: number): string {
	return CE_ARMOR_COLORS[index]?.hex ?? DEFAULT_ARMOR;
}

function accuracyOf(shotsFired: number, shotsHit: number): number {
	if (!shotsFired || shotsFired <= 0) return 0;
	return (shotsHit / shotsFired) * 100;
}

/**
 * localOverlayPlayers maps cartographer's LOCAL roster (joined to the tick by
 * player index for live health/shield/alive/camo/respawn) into the overlay's
 * PlayerCard shape, ordered by local_index (viewport order). Its length is the
 * split count. Fields cartographer doesn't expose (damage ratio) default; the
 * card degrades gracefully.
 */
export function localOverlayPlayers(
	game: GamePayload | null,
	tick: TickPayloadV2 | null
): OverlayPlayer[] {
	const isTeamGame = game?.config?.is_team_game === true;
	const ticks = tick?.players ?? [];
	const tickByIndex = new Map(ticks.map((t) => [t.index, t]));

	return localRoster(game).map((p) => {
		const t = tickByIndex.get(p.index);
		const respawnTicks = t?.respawn_in_ticks ?? null;
		return {
			slot: p.local_index ?? 0,
			name: p.name,
			team: teamOf(isTeamGame, p.team),
			armor: armorHex(p.armor_color),
			score: p.score,
			kills: p.kills,
			deaths: p.deaths,
			assists: p.assists,
			spree: p.kill_streak,
			accuracy: accuracyOf(p.shots_fired, p.shots_hit),
			damageRatio: 0,
			alive: t?.alive ?? true,
			respawn: respawnTicks != null ? Math.ceil(respawnTicks / TICKS_PER_SECOND) : 0,
			respawnMax: RESPAWN_MAX,
			camo: t?.has_camo ? CAMO_MAX : 0,
			camoMax: CAMO_MAX,
			shield: t?.shields ?? 1,
			health: t?.health ?? 1
		};
	});
}
