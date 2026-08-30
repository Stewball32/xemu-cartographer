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
// Field availability note (updated 2026-08-15 for the new graphics pack):
//
// WIRED + verified non-zero — score, kills, deaths, betrayals (=team_kills),
// suicides, spree (=kill_streak, CURRENT streak) and bestSpree
// (=best_kill_streak, match PEAK). Match clock = engine_tick (0x0C, verified
// counting at 30Hz); it free-runs at the menu, so it is phase-gated here and
// latched for the post-game duration (see createClockLatch).
//
// WIRED via the server-side accumulator (acc_* — the HaloCaster extract_events
// port in internal/scraper/accum.go, which counts from per-tick deltas because
// the engine's own counters read 0): shots fired, grenade throws, melees,
// damage dealt/received, camo + overshield pickups.
//
// STILL UNAVAILABLE, rendered as an em dash rather than a fake 0:
//   • accuracy — needs shots_HIT, which reads 0 live; only shots_fired is
//     counted, so a hit rate cannot be derived at all.
//   • headshots — not on the wire in any form (offset-hunt candidate).
// Also deferred: a real camo timer (only the has_camo bool, so the cloak
// animation runs on a fixed 30s assumption) and team names (the wire carries
// numeric ids only → RED TEAM / BLUE TEAM).

import type { GamePayload, ScenarioPayload, TickPayloadV2 } from '$lib/types/scraper-v2';
import { damageRatioOf, type OverlayPlayer } from '$lib/utils/overlay-split';
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
		// CE pads names out of the profile block, so trim before anything keys off
		// it — this is the identity-lookup key and the leaderboard's {#each} key.
		const name = p.name.trim();
		return {
			slot: p.local_index ?? p.index,
			name,
			display: name,
			avatar: null,
			team: teamOf(isTeamGame, p.team),
			armor: armorHex(p.armor_color),
			score: p.score,
			kills: p.kills,
			deaths: p.deaths,
			assists: p.assists,
			spree: p.kill_streak,
			bestSpree: p.best_kill_streak ?? 0,
			accuracy: accuracyOf(p.shots_fired, p.shots_hit),
			betrayals: p.team_kills, // verified populated live (beta-stream)
			suicides: p.suicides, // verified populated live (beta-stream)
			// Accumulated match stats (acc_* wire fields — server-side deltas).
			shotsFired: p.acc_shots_fired ?? 0,
			grenadeThrows: p.acc_grenade_throws ?? 0,
			meleeKills: p.acc_melees ?? 0,
			damageDealt: p.acc_damage_dealt ?? 0,
			damageTaken: p.acc_damage_received ?? 0,
			damageRatio: damageRatioOf(p.acc_damage_dealt ?? 0, p.acc_damage_received ?? 0),
			camoPickups: p.acc_camo_pickups ?? 0,
			osPickups: p.acc_overshield_pickups ?? 0,
			alive: t?.alive ?? true,
			// Unrounded: RespawnRing sweeps its arc off this value and ceils
			// separately for the digit, so rounding here made the arc step in
			// whole seconds instead of draining.
			respawn: respawnTicks != null ? respawnTicks / TICKS_PER_SECOND : 0,
			respawnMax: RESPAWN_MAX,
			// The scraper exposes camo as a boolean, and PlayerCard reads any
			// value > 1 as "percent remaining" — so CAMO_MAX here pinned the
			// wipe at a constant 30%. 1 selects the nominal decay instead.
			camo: t?.has_camo ? 1 : 0,
			camoMax: CAMO_MAX,
			shield: t?.shields ?? 1,
			health: t?.health ?? 1
		};
	});
}

/** One resolved broadcast identity from GET /api/public/profiles. */
export interface OverlayIdentity {
	/** The player's default gamertag — the handle to put on air. */
	display?: string;
	/** Absolute avatar URL, already origin-resolved by the lookup store. */
	avatar?: string;
	/** The plate's second line (users.motto — settings Stream tab). */
	motto?: string;
	/** Absolute 600×100 banner-art URL of the picked nameplate, origin-resolved
	 * by the lookup store. Renders under the plate's navy scrim. */
	plate?: string;
}

/**
 * applyIdentities overlays resolved identities (and any manual ?names= override)
 * onto a mapped roster, setting `display` and `avatar`.
 *
 * Precedence, highest first:
 *   1. ?names=SCRAPED:Display — the operator's manual override always wins, so a
 *      name can be forced on air regardless of what the lookup says.
 *   2. The resolved default gamertag.
 *   3. The trimmed scraped name.
 *
 * `name` is never rewritten — see the OverlayPlayer doc comment. Matching is on
 * the lowercased scraped name, the same key the endpoint returns.
 */
export function applyIdentities(
	players: OverlayPlayer[],
	identities: Record<string, OverlayIdentity> = {},
	nameOverrides: Record<string, string> = {}
): OverlayPlayer[] {
	return players.map((p) => {
		const id = identities[p.name.toLowerCase()];
		const override = nameOverrides[p.name];
		return {
			...p,
			display: override ?? id?.display ?? p.name,
			avatar: id?.avatar ?? null,
			motto: id?.motto ?? '',
			plateBg: id?.plate ?? ''
		};
	});
}

export interface OverlayMatchTeam {
	id: 'red' | 'blue';
	name: string; // wire carries only numeric ids → falls back to RED/BLUE TEAM
	score: number;
}

export interface OverlayMatch {
	/** Drives the FFA-vs-team layout switch in every graphic. */
	mode: 'ffa' | 'team';
	gametype: string;
	map: string;
	clock?: string; // undefined outside a live match (engine_tick free-runs at the menu)
	phase: 'live' | 'postgame';
	killLimit?: number;
	/** Post-game banner line, e.g. `SLAYER — KILL LIMIT 15`. */
	rules: string;
	teams: OverlayMatchTeam[] | null; // null = FFA
}

const TEAM_NAMES: Record<'red' | 'blue', string> = {
	red: 'RED TEAM',
	blue: 'BLUE TEAM'
};

/** Xbox map tag paths arrive like `levels\test\bloodgulch\bloodgulch` — reduce
 * to the last path segment, upper-cased, for a clean overlay label. */
function cleanMapName(raw: string): string {
	const seg = raw.split(/[\\/]/).filter(Boolean).pop() ?? raw;
	return seg.toUpperCase();
}

/** Halo CE match clock: a 30Hz count-up tick → M:SS (e.g. 8:42), counting up
 * from 0:00 at match start (CE has no countdown). */
export function ticksToClock(ticks: number): string {
	const sec = Math.max(0, Math.floor(ticks / TICKS_PER_SECOND));
	return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;
}

/** Post-game banner rules line. CE's score limit means a different thing per
 * gametype, so name it accordingly rather than always saying "kill limit". */
function rulesLine(gametype: string, scoreLimit: number | undefined): string {
	const gt = gametype.toUpperCase();
	if (!gt) return '';
	if (!scoreLimit || scoreLimit <= 0) return gt;
	const noun =
		gt === 'CTF'
			? 'FLAG LIMIT'
			: gt === 'ODDBALL'
				? 'TARGET TIME'
				: gt === 'KING' || gt === 'KING OF THE HILL'
					? 'TARGET TIME'
					: gt === 'RACE'
						? 'LAP LIMIT'
						: 'KILL LIMIT';
	return `${gt} — ${noun} ${scoreLimit}`;
}

/** Rank order shared by every graphic: score desc, then kills desc, then fewest
 * deaths. The pack's contract puts sorting on the overlay, not the scraper, so
 * players arrive in roster order and each view ranks them itself. */
export function rankPlayers(players: OverlayPlayer[]): OverlayPlayer[] {
	return [...players].sort((a, b) => b.score - a.score || b.kills - a.kills || a.deaths - b.deaths);
}

const ORDINALS = ['1ST', '2ND', '3RD', '4TH', '5TH', '6TH', '7TH', '8TH'];

/** Zero-based placing → `1ST`…`8TH`, em dash past the CE lobby cap. */
export function ordinal(placeIndex: number): string {
	return ORDINALS[placeIndex] ?? '—';
}

/**
 * Final-duration latch for the post-game banner.
 *
 * `engine_tick` (GTG 0x0C) is the real match clock while a match runs, but it
 * FREE-RUNS once the game drops back to the menu — so by the time the post-game
 * report renders, reading it directly would show a number that keeps climbing.
 * The overlay therefore remembers the last tick it saw while `phase === 'live'`
 * and reports that as the match duration.
 *
 * Kept Svelte-free (plain closure, no runes) so it unit-tests like the rest of
 * this module; the page drives it from an `$effect`. An OBS browser source left
 * warm across the match end — which the pack's README already tells operators to
 * do — will have observed the live phase and latched a real duration. A source
 * that only starts after the match ends has nothing to latch, hence `seen`.
 */
export function createClockLatch() {
	let ticks = 0;
	let seen = false;
	return {
		/** Feed every game snapshot; only live ones move the latch. */
		observe(game: GamePayload | null): void {
			if (game?.phase !== 'live') return;
			ticks = game.engine_tick ?? 0;
			seen = true;
		},
		/** M:SS of the final live tick, or undefined if no live match was seen. */
		get duration(): string | undefined {
			return seen ? ticksToClock(ticks) : undefined;
		},
		get ticks(): number {
			return ticks;
		}
	};
}

export interface OverlayTotals {
	kills: number;
	shots: number;
	nades: number;
	damage: string; // pre-formatted, e.g. `48.2K`
}

/** Lobby-wide totals for the post-game footer. Summed client-side from the
 * player rows per the pack contract — the scraper never sends these. */
export function matchTotals(players: OverlayPlayer[]): OverlayTotals {
	const sum = (pick: (p: OverlayPlayer) => number) => players.reduce((n, p) => n + pick(p), 0);
	return {
		kills: sum((p) => p.kills),
		shots: sum((p) => p.shotsFired),
		nades: sum((p) => p.grenadeThrows),
		damage: compactNumber(sum((p) => p.damageDealt))
	};
}

/** 48231 → `48.2K`. Keeps the footer on one line at any match length. */
export function compactNumber(n: number): string {
	const v = Math.round(n);
	if (Math.abs(v) >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
	if (Math.abs(v) >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
	return String(v);
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
				.map((ts) => {
					const id = TEAM_IDS[ts.team] ?? 'red';
					return { id, name: TEAM_NAMES[id], score: ts.score };
				})
		: null;
	// Count-up match clock. LIVE-VERIFIED on beta-stream (2026-08-08): engine_tick
	// (GTG 0x0C) counts up at 30Hz during a match and re-inits to 0 at match
	// start, while game_elapsed_ticks (0x10) is stuck at ~1 and never ticks — so
	// engine_tick is the real match clock, NOT 0x10 (the offset-mapper's inferred
	// field). Gated to a live match so 0x0C's menu free-running value never shows.
	const clock = game?.phase === 'live' ? ticksToClock(game.engine_tick ?? 0) : undefined;
	const gametype = (cfg?.gametype ?? '').toUpperCase();
	return {
		mode: isTeamGame ? 'team' : 'ffa',
		gametype,
		map: cleanMapName(scenario?.map ?? ''),
		clock,
		phase: game?.phase === 'idle' ? 'postgame' : 'live',
		killLimit: cfg?.score_limit,
		rules: rulesLine(gametype, cfg?.score_limit),
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
		// ?anchor=center centres the graphic in a larger OBS source box instead of
		// pinning it to the top-left at its natural size.
		anchor: p.get('anchor') === 'center' ? 'center' : null,
		names: Object.fromEntries(
			(p.get('names') ?? '')
				.split(',')
				.filter(Boolean)
				.map((pair) => pair.split(':'))
		) as Record<string, string>
	};
}
