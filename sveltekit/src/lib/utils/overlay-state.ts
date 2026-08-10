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
// Field availability note (2026-08-08, live-audited against beta-stream slayer):
// WIRED + verified non-zero on the wire — score, kills, deaths, betrayals
// (=team_kills), suicides, spree (=kill_streak, CURRENT streak not peak).
// PRESENT but read 0 even mid-match, so NOT trustworthy yet (offset-hunt):
// accuracy (shots_fired/shots_hit both 0 at 22 kills) and assists (0 across the
// board — real-0 vs dead-offset unknown without forcing one). NOT on the wire at
// all (offset-hunt candidates): damage, headshots, melee kills, grenade kills,
// camo pickups, overshield pickups. Match clock = engine_tick (0x0C, verified
// counting). Real camo timer + team names still deferred (camo from has_camo
// bool; team names fall back to RED/BLUE TEAM). See the overlay reboot plan +
// the stats gap list.

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
 * overlayPlayers maps the roster into the pack's OverlayPlayer shape, joined to
 * the tick by player index for live health/shield/alive/respawn. By default it
 * maps the FULL roster (whole-match overlays — scorebug/leaderboard sort/group
 * themselves). Pass `machineIndex` to keep ONLY the players seated on that
 * system-link machine — the per-console (POV) case: the resolver hands the
 * console name → its current machine index (indices shift live, so this is
 * evaluated per snapshot), and this returns that console's own player(s).
 */
export function overlayPlayers(
	game: GamePayload | null,
	tick: TickPayloadV2 | null,
	machineIndex?: number | null
): OverlayPlayer[] {
	const isTeamGame = game?.config?.is_team_game === true;
	const ticks = tick?.players ?? [];
	const tickByIndex = new Map(ticks.map((t) => [t.index, t]));

	const roster =
		machineIndex == null || machineIndex < 0
			? (game?.players ?? [])
			: (game?.players ?? []).filter((p) => p.machine_index === machineIndex);

	return roster.map((p) => {
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
			bestSpree: p.best_kill_streak ?? 0,
			accuracy: accuracyOf(p.shots_fired, p.shots_hit),
			damageRatio: 0,
			betrayals: p.team_kills, // verified populated live (beta-stream)
			suicides: p.suicides, // verified populated live (beta-stream)
			// Accumulated match stats (acc_* wire fields — server-side deltas).
			shotsFired: p.acc_shots_fired ?? 0,
			grenadeThrows: p.acc_grenade_throws ?? 0,
			meleeKills: p.acc_melees ?? 0,
			damageDealt: p.acc_damage_dealt ?? 0,
			damageTaken: p.acc_damage_received ?? 0,
			camoPickups: p.acc_camo_pickups ?? 0,
			osPickups: p.acc_overshield_pickups ?? 0,
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

/** Halo CE match clock: a 30Hz count-up tick → M:SS (e.g. 8:42), counting up
 * from 0:00 at match start (CE has no countdown). */
function ticksToClock(ticks: number): string {
	const sec = Math.max(0, Math.floor(ticks / TICKS_PER_SECOND));
	return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;
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
	// Count-up match clock. LIVE-VERIFIED on beta-stream (2026-08-08): engine_tick
	// (GTG 0x0C) counts up at 30Hz during a match and re-inits to 0 at match
	// start, while game_elapsed_ticks (0x10) is stuck at ~1 and never ticks — so
	// engine_tick is the real match clock, NOT 0x10 (the offset-mapper's inferred
	// field). Gated to a live match so 0x0C's menu free-running value never shows.
	const clock = game?.phase === 'live' ? ticksToClock(game.engine_tick ?? 0) : undefined;
	return {
		gametype: (cfg?.gametype ?? '').toUpperCase(),
		map: cleanMapName(scenario?.map ?? ''),
		clock,
		phase: game?.phase === 'idle' ? 'postgame' : 'live',
		killLimit: cfg?.score_limit,
		teams
	};
}

/** Shared loader params for the native OBS overlay routes (?console= target +
 * optional mock + display-name overrides). Mirrors the POV overlay's loader so
 * all four routes take the same URL shape. */
export function nativeOverlayParams(url: URL) {
	const p = url.searchParams;
	const mock = p.get('mock');
	return {
		// PoC: `console` targets by console name alone (no instance/token) — the
		// feed resolves it to whichever host currently sees that console.
		console: p.get('console') ?? '',
		mock: mock === '1' || mock === 'true',
		// ?transport=poll forces the HTTP-poll console fallback (default is WS push).
		consolePoll: p.get('transport') === 'poll',
		names: Object.fromEntries(
			(p.get('names') ?? '')
				.split(',')
				.filter(Boolean)
				.map((pair) => pair.split(':'))
		) as Record<string, string>
	};
}
