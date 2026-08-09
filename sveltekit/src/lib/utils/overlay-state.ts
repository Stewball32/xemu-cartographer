// Native-feed mappers for the full-roster overlays (scorebug / leaderboard /
// postgame). These are the counterpart to overlay-split.ts (which maps only the
// host-local splitscreen roster for the POV overlay): here we map the ENTIRE
// roster plus the match-level state the OBS pack's scorebug/leaderboard/postgame
// components expect, sourced from cartographer's native wire classes
// (game / tick / scenario) instead of the pack's foreign ws://…:8765 feed.
//
// Pure (no Svelte / no socket) so it's unit-testable; the pages wire it to the
// reactive createOverlayFeed().
//
// Field availability note (2026-08-08): live match clock, real camo timer, team
// names, and the full postgame carnage ledger are NOT on the wire yet, so those
// degrade gracefully (clock omitted → pack shows a placeholder; camo from the
// has_camo bool; team names fall back to RED/BLUE TEAM; postgame renders the
// live stats we have). See the overlay reboot plan.

import type { GamePayload, ScenarioPayload, TickPayloadV2 } from '$lib/types/scraper-v2';
import type { OverlayPlayer } from '$lib/utils/overlay-split';
import { CE_ARMOR_COLORS } from '$lib/data/halo-armor-palettes';
import { TICKS_PER_SECOND } from '$lib/utils/overlay-view';

const RESPAWN_MAX = 5;
const CAMO_MAX = 30;
const DEFAULT_ARMOR = '#9fb4d0';
const TEAM_IDS = ['red', 'blue'] as const;

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
 * overlayPlayers maps the FULL roster (every player, not just splitscreen
 * locals) into the pack's OverlayPlayer shape, joined to the tick by player
 * index for live health/shield/alive/respawn. Ordered as the roster arrives;
 * the leaderboard/scorebug sort by score themselves.
 */
export function overlayPlayers(
	game: GamePayload | null,
	tick: TickPayloadV2 | null
): OverlayPlayer[] {
	const isTeamGame = game?.config?.is_team_game === true;
	const ticks = tick?.players ?? [];
	const tickByIndex = new Map(ticks.map((t) => [t.index, t]));

	return (game?.players ?? []).map((p) => {
		const t = tickByIndex.get(p.index);
		const respawnTicks = t?.respawn_in_ticks ?? null;
		return {
			slot: p.local_index ?? p.index,
			name: p.name,
			team: teamOf(isTeamGame, p.team),
			armor: armorHex(p.armor_color),
			score: p.score,
			kills: p.kills,
			deaths: p.deaths,
			assists: p.assists,
			spree: p.kill_streak,
			accuracy: accuracyOf(p.shots_fired, p.shots_hit),
			damageRatio: 0, // not on the wire (deferred)
			alive: t?.alive ?? true,
			respawn: respawnTicks != null ? Math.ceil(respawnTicks / TICKS_PER_SECOND) : 0,
			respawnMax: RESPAWN_MAX,
			camo: t?.has_camo ? CAMO_MAX : 0, // real timer deferred
			camoMax: CAMO_MAX,
			shield: t?.shields ?? 1,
			health: t?.health ?? 1
		};
	});
}

export interface OverlayMatchTeam {
	id: 'red' | 'blue';
	name?: string; // wire carries only numeric ids → pack falls back to RED/BLUE TEAM
	score: number;
}

export interface OverlayMatch {
	gametype: string;
	map: string;
	clock?: string; // live clock not on the wire yet (deferred) → pack shows a placeholder
	phase: 'live' | 'postgame';
	killLimit?: number;
	teams: OverlayMatchTeam[] | null; // null = FFA
}

/** Xbox map tag paths arrive like `levels\test\bloodgulch\bloodgulch` — reduce
 * to the last path segment, upper-cased, for a clean overlay label. */
function cleanMapName(raw: string): string {
	const seg = raw.split(/[\\/]/).filter(Boolean).pop() ?? raw;
	return seg.toUpperCase();
}

/**
 * matchState maps the native game/scenario classes into the pack's match object
 * (team panels / gametype / map / kill limit). FFA → teams:null.
 */
export function matchState(
	game: GamePayload | null,
	scenario: ScenarioPayload | null
): OverlayMatch {
	const cfg = game?.config ?? null;
	const isTeamGame = cfg?.is_team_game === true;
	const teams: OverlayMatchTeam[] | null = isTeamGame
		? (game?.team_scores ?? [])
				.filter((ts) => ts.team === 0 || ts.team === 1)
				.map((ts) => ({ id: TEAM_IDS[ts.team] ?? 'red', score: ts.score }))
		: null;
	return {
		gametype: (cfg?.gametype ?? '').toUpperCase(),
		map: cleanMapName(scenario?.map ?? ''),
		phase: game?.phase === 'idle' ? 'postgame' : 'live',
		killLimit: cfg?.score_limit,
		teams
	};
}

/** Shared loader params for the native OBS overlay routes (instance + read-only
 * overlay token + optional mock + display-name overrides). Mirrors the POV
 * overlay's loader so all four routes take the same URL shape. */
export function nativeOverlayParams(url: URL) {
	const p = url.searchParams;
	const mock = p.get('mock');
	return {
		instance: p.get('instance') ?? '',
		token: p.get('token') ?? '',
		mock: mock === '1' || mock === 'true',
		names: Object.fromEntries(
			(p.get('names') ?? '')
				.split(',')
				.filter(Boolean)
				.map((pair) => pair.split(':'))
		) as Record<string, string>
	};
}
