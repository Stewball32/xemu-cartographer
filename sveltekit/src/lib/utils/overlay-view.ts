// Pure view-model builders for the OBS spectator overlays. These take the raw
// v2 `game` + `tick` (+ `scenario`) payloads off the WS feed and project them
// into render-ready shapes. Kept IO-free so they unit-test without a socket.
//
// Trust boundary (see docs/milestones/M25-obs-overlays.md):
//   - RUNTIME-VERIFIED: player names, teams, alive status, health/shields,
//     object/world data, map name.
//   - BEST-EFFORT (not yet runtime-verified — pending a live play session):
//     per-player kills/deaths/assists/score and team scores. The builders
//     surface these but flag them via `hasScores` so the UI can degrade
//     gracefully (mute or hide) instead of asserting numbers we can't trust.

import type {
	AnyEvent,
	DeathCause,
	DeathEvent,
	GamePayload,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';

/** Halo: CE biped health / shield ceilings — both 75 engine units
 * (RUNTIME-VERIFIED 2026-06-21 via OffDynMaxHealth 0x88 / OffDynMaxShields 0x8C).
 *
 * IMPORTANT: the wire `health` / `shields` fields already arrive normalised
 * 0..1 (the engine's body_vitality fraction — 1.0 = full, RUNTIME-VERIFIED:
 * 1.0 → 0.625 → 0.437 under fire). Bar widths therefore use them DIRECTLY and
 * must NOT divide by these ceilings — doing so rendered a full-health bar at
 * ~1.3% width against live data. These constants remain for absolute-HP display
 * (absolute HP = fraction * ceiling), corroborated by the wire's new
 * max_health / max_shields fields. */
export const HEALTH_MAX = 75;
export const SHIELD_MAX = 75;

/** Team index → display name + accent colour. Indices beyond the table wrap
 * (defensive — Halo: CE tops out at 4 teams for the shipped gametypes). */
const TEAM_META = [
	{ name: 'Red', color: '#e3413f' },
	{ name: 'Blue', color: '#2f74ec' },
	{ name: 'Green', color: '#36b55a' },
	{ name: 'Gold', color: '#e0a32e' },
	{ name: 'Purple', color: '#9b51e0' },
	{ name: 'Orange', color: '#e07b2e' },
	{ name: 'Cyan', color: '#27c0c4' },
	{ name: 'Pink', color: '#e35aa8' }
] as const;

export interface TeamMeta {
	name: string;
	color: string;
}

export function teamMeta(team: number): TeamMeta {
	const len = TEAM_META.length;
	const idx = ((Math.trunc(team) % len) + len) % len;
	return TEAM_META[idx];
}

function clamp01(n: number): number {
	if (!Number.isFinite(n)) return 0;
	if (n < 0) return 0;
	if (n > 1) return 1;
	return n;
}

/** Title-case a tag/identifier for display: strips path segments, swaps
 * underscores/hyphens for spaces, capitalises words. "team_slayer" → "Team
 * Slayer"; "levels\\test\\bloodgulch\\bloodgulch" → "Bloodgulch". */
export function prettify(raw: string | null | undefined): string {
	if (!raw) return '';
	const segs = raw.split(/[\\/]/);
	const base = segs[segs.length - 1] || raw;
	return base
		.replace(/[_-]+/g, ' ')
		.trim()
		.replace(/\b\w/g, (c) => c.toUpperCase());
}

export interface PlayerRow {
	index: number;
	name: string;
	team: number;
	isLocal: boolean;
	/** Live (tick) — only meaningful when the scoreboard's `hasTick` is true. */
	alive: boolean;
	respawnIn: number | null;
	/** 0..1 engine body_vitality fraction (clamped; shields >1 with overshield). */
	health: number;
	shields: number;
	hasOvershield: boolean;
	hasCamo: boolean;
	/** Best-effort cumulative stats from the game roster — UNVERIFIED. */
	kills: number;
	deaths: number;
	assists: number;
	score: number;
}

export interface TeamGroup {
	team: number;
	name: string;
	color: string;
	/** Best-effort team score (team_scores entry, else summed member score). */
	score: number;
	players: PlayerRow[];
}

export interface ScoreboardVM {
	phase: string;
	gametype: string;
	isTeamGame: boolean;
	/** True once a tick payload has arrived — gates health bars / alive dimming. */
	hasTick: boolean;
	/** True when any K/D/A/score/team-score is non-zero. When false the score
	 * columns are absent (or zeroed) and the UI should mute them. */
	hasScores: boolean;
	/** Populated when isTeamGame. */
	teams: TeamGroup[];
	/** Flat, sorted; populated when !isTeamGame (FFA). */
	players: PlayerRow[];
	playerCount: number;
}

function joinRow(
	p: GamePayload['players'][number],
	tickByIndex: Map<number, TickPayloadV2['players'][number]>
): PlayerRow {
	const t = tickByIndex.get(p.index) ?? null;
	return {
		index: p.index,
		name: p.name,
		team: p.team,
		isLocal: p.is_local === true,
		alive: t ? t.alive : true,
		respawnIn: t ? t.respawn_in_ticks : null,
		// Wire health/shields are already the engine's 0..1 fraction
		// (RUNTIME-VERIFIED) — use directly, do NOT divide by HEALTH_MAX.
		// Shields can read >1 with overshield; clamp for the bar (the
		// has_overshield flag carries the over-full state).
		health: t ? clamp01(t.health) : 0,
		shields: t ? clamp01(t.shields) : 0,
		hasOvershield: t?.has_overshield ?? false,
		hasCamo: t?.has_camo ?? false,
		kills: p.kills ?? 0,
		deaths: p.deaths ?? 0,
		assists: p.assists ?? 0,
		score: p.score ?? 0
	};
}

/** Sort key for scoreboard ordering: score desc, then kills desc, then name. */
function byStanding(a: PlayerRow, b: PlayerRow): number {
	if (b.score !== a.score) return b.score - a.score;
	if (b.kills !== a.kills) return b.kills - a.kills;
	return a.name.localeCompare(b.name);
}

/** buildScoreboard joins the roster (identity + cumulative stats) with the
 * latest tick (alive / health / shields) into a render-ready scoreboard. All
 * named players are included (not just split-screen locals) so a system-link
 * match shows every console's players. Degrades cleanly: no game → empty VM;
 * no tick → roster with hasTick=false (no bars). */
export function buildScoreboard(
	game: GamePayload | null | undefined,
	tick: TickPayloadV2 | null | undefined
): ScoreboardVM {
	const empty: ScoreboardVM = {
		phase: game?.phase ?? 'idle',
		gametype: prettify(game?.config?.variant_name || game?.config?.gametype),
		isTeamGame: false,
		hasTick: false,
		hasScores: false,
		teams: [],
		players: [],
		playerCount: 0
	};
	if (!game || !Array.isArray(game.players)) return empty;

	const tickByIndex = new Map<number, TickPayloadV2['players'][number]>();
	if (tick?.players) {
		for (const tp of tick.players) tickByIndex.set(tp.index, tp);
	}
	const hasTick = tick != null && Array.isArray(tick.players);

	const rows = game.players
		.filter((p) => (p.name ?? '').trim().length > 0)
		.map((p) => joinRow(p, tickByIndex));

	if (rows.length === 0) return { ...empty, hasTick };

	// Prefer the engine's explicit team flag; otherwise infer from a spread of
	// distinct team indices.
	const distinctTeams = new Set(rows.map((r) => r.team));
	const isTeamGame = game.config ? game.config.is_team_game === true : distinctTeams.size > 1;

	const teamScoreByIndex = new Map<number, number>();
	for (const ts of game.team_scores ?? []) teamScoreByIndex.set(ts.team, ts.score ?? 0);

	const playerScoreSeen = rows.some(
		(r) => r.kills !== 0 || r.deaths !== 0 || r.assists !== 0 || r.score !== 0
	);
	const teamScoreSeen = [...teamScoreByIndex.values()].some((s) => s !== 0);
	const hasScores = playerScoreSeen || teamScoreSeen;

	if (!isTeamGame) {
		return {
			...empty,
			isTeamGame: false,
			hasTick,
			hasScores,
			players: [...rows].sort(byStanding),
			playerCount: rows.length
		};
	}

	const groups = new Map<number, PlayerRow[]>();
	for (const r of rows) {
		const arr = groups.get(r.team);
		if (arr) arr.push(r);
		else groups.set(r.team, [r]);
	}
	const teams: TeamGroup[] = [...groups.entries()]
		.sort((a, b) => a[0] - b[0])
		.map(([team, members]) => {
			const meta = teamMeta(team);
			const score = teamScoreByIndex.get(team) ?? members.reduce((sum, m) => sum + m.score, 0);
			return {
				team,
				name: meta.name,
				color: meta.color,
				score,
				players: [...members].sort(byStanding)
			};
		});

	return {
		...empty,
		isTeamGame: true,
		hasTick,
		hasScores,
		teams,
		playerCount: rows.length
	};
}

export interface StatusTeam {
	team: number;
	name: string;
	color: string;
	score: number;
}

export interface StatusVM {
	phase: string;
	gametype: string;
	variant: string;
	map: string;
	isTeamGame: boolean;
	scoreLimit: number;
	/** Best-effort team scores (joined to team metadata), ordered by index. */
	teams: StatusTeam[];
	/** Best-effort FFA leader score. */
	topScore: number;
	hasScores: boolean;
}

/** statusStrip projects the slow-changing match header — gametype, variant,
 * map, and (best-effort) team scores — for the compact status-bar overlay. */
export function statusStrip(
	game: GamePayload | null | undefined,
	scenario: ScenarioPayload | null | undefined
): StatusVM {
	const cfg = game?.config ?? null;
	const isTeamGame = cfg ? cfg.is_team_game === true : false;

	const teams: StatusTeam[] = (game?.team_scores ?? [])
		.slice()
		.sort((a, b) => a.team - b.team)
		.map((ts) => {
			const meta = teamMeta(ts.team);
			return { team: ts.team, name: meta.name, color: meta.color, score: ts.score ?? 0 };
		});

	const playerTop = Math.max(0, ...(game?.players ?? []).map((p) => p.score ?? 0));
	const teamTop = Math.max(0, ...teams.map((t) => t.score));
	const hasScores = teams.some((t) => t.score !== 0) || playerTop !== 0;

	return {
		phase: game?.phase ?? 'idle',
		gametype: prettify(cfg?.gametype),
		variant: cfg?.variant_name ?? '',
		map: prettify(scenario?.map),
		isTeamGame,
		scoreLimit: cfg?.score_limit ?? 0,
		teams,
		topScore: Math.max(playerTop, teamTop),
		hasScores
	};
}

// =============================================================================
// Match clock (game-timer overlay)
// =============================================================================

/** Halo: CE engine tick rate — the simulation steps at 30 Hz, so engine_tick /
 * 30 is the match time in seconds. (Established engine constant; corroborated by
 * the respawn_in_ticks → seconds math the scoreboard already does, ÷30.) */
export const TICKS_PER_SECOND = 30;

/** Format a non-negative second count as `M:SS` (e.g. 482 → "8:02"). Minutes are
 * unpadded (scoreboard-clock convention); seconds always two digits. */
export function mmss(totalSeconds: number): string {
	const s = Number.isFinite(totalSeconds) ? Math.max(0, Math.floor(totalSeconds)) : 0;
	const m = Math.floor(s / 60);
	const sec = s % 60;
	return `${m}:${sec.toString().padStart(2, '0')}`;
}

export interface ClockVM {
	phase: string;
	/** True once the match is Live — drives the pulsing "LIVE" treatment. */
	live: boolean;
	/** 'up' counts elapsed time; 'down' counts toward a configured time limit. */
	direction: 'up' | 'down';
	/** M:SS display — elapsed when counting up, remaining when counting down. */
	label: string;
	elapsedSeconds: number;
	/** null when the gametype has no time limit (count-up). */
	remainingSeconds: number | null;
	gametype: string;
	scoreLimit: number;
	hasGame: boolean;
}

/** matchClock projects the match timing off the game payload: elapsed time from
 * `engine_tick` (÷ 30 Hz), counting down toward a time limit when the gametype
 * sets one, else up. Pure + degrades to an idle 0:00 with no game. */
export function matchClock(game: GamePayload | null | undefined): ClockVM {
	const cfg = game?.config ?? null;
	const phase = game?.phase ?? 'idle';
	const elapsedTicks = Math.max(0, game?.engine_tick ?? 0);
	const elapsedSeconds = elapsedTicks / TICKS_PER_SECOND;
	const limitTicks = cfg?.time_limit_ticks ?? 0;
	const hasLimit = limitTicks > 0;
	const remainingSeconds = hasLimit
		? Math.max(0, (limitTicks - elapsedTicks) / TICKS_PER_SECOND)
		: null;
	const direction: 'up' | 'down' = hasLimit ? 'down' : 'up';
	return {
		phase,
		live: phase === 'live',
		direction,
		label: mmss(direction === 'down' ? (remainingSeconds ?? 0) : elapsedSeconds),
		elapsedSeconds,
		remainingSeconds,
		gametype: prettify(cfg?.variant_name || cfg?.gametype),
		scoreLimit: cfg?.score_limit ?? 0,
		hasGame: !!game
	};
}

// =============================================================================
// Kill feed
// =============================================================================

export interface KillRow {
	/** Stable de-dupe / keyed-each identity (seq + tick). */
	key: string;
	tick: number;
	/** null when there is no killer (suicide / fall / environment). */
	killerName: string | null;
	killerColor: string;
	victimName: string;
	victimColor: string;
	/** Prettified weapon name; '' for environmental / fall deaths. */
	weapon: string;
	cause: DeathCause;
	teamKill: boolean;
	/** Suicide / fall / environment / no killer — render as a self-death. */
	selfInflicted: boolean;
}

export interface KillFeedVM {
	rows: KillRow[];
	count: number;
}

function isDeathEvent(e: AnyEvent): e is DeathEvent {
	return e.event_type === 'death';
}

/** buildKillFeed projects the rolling event log (newest-first, as the WS store
 * keeps it) into the most recent N death rows for the kill-feed overlay. Identity
 * is read straight off the denormalised event PlayerRefs, so it needs no roster
 * join. Non-death events are ignored; degrades to an empty feed. */
export function buildKillFeed(events: AnyEvent[] | null | undefined, max = 5): KillFeedVM {
	const deaths = (events ?? []).filter(isDeathEvent);
	const rows: KillRow[] = deaths.slice(0, Math.max(0, max)).map((d) => {
		const selfInflicted = d.killer == null || d.cause === 'suicide' || d.cause === 'fall';
		return {
			key: `${d.seq}-${d.tick}`,
			tick: d.tick,
			killerName: d.killer ? d.killer.name : null,
			killerColor: d.killer ? teamMeta(d.killer.team).color : '#9aa4b2',
			victimName: d.victim.name,
			victimColor: teamMeta(d.victim.team).color,
			weapon: prettify(d.weapon),
			cause: d.cause,
			teamKill: d.team_kill === true,
			selfInflicted
		};
	});
	return { rows, count: rows.length };
}
