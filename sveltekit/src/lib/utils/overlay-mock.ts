// Sample-data mode for the OBS overlays. Lets Stewart preview the overlays
// rendering with a believable 4v4 Team Slayer on Bloodgulch — no live xemu, no
// minted token. Enabled with ?mock=1 on any overlay URL.
//
// The mock is a CHOREOGRAPHY, not a set of wobbling constants. A single
// deterministic kill script is forward-simulated from frame 0 and every
// frame-dependent export (`mockGame`, `mockTick`, `mockEvents`) reads the same
// simulated state, so the scoreboard, kill feed, sprees, respawn rings and team
// scores can never disagree with each other. That matters because most of the
// overlay pack's motion is DATA-driven — a respawn ring only drains if someone
// is actually respawning, a spree badge only cascades if a streak actually
// breaks — so a static fixture previews the layout but none of the animation.
//
// One cycle is a full match followed by a short idle gap, then a complete
// reset. The idle window is what drives the match-end out-animations (the pages
// latch on live→not-live), and the reset is what keeps scores from creeping
// past the score limit and lets the team-lead flip happen every time around.

import type {
	AnyEvent,
	DeathCause,
	DeathEvent,
	GamePayload,
	ObjectsPayload,
	PlayerRef,
	ScenarioPayload,
	TickPayloadV2
} from '$lib/types/scraper-v2';
import { CE_COLORS, colorHex, H2_KEYS, type Appearance } from '$lib/utils/emblem';

/** Instance name the mock pretends to be. Any instance segment works in mock
 * mode — the URL's `[instance]` is ignored when ?mock=1 is set. */
export const MOCK_INSTANCE = 'demo';

// ---------------------------------------------------------------------------
// Cadence
// ---------------------------------------------------------------------------
// The feed advances `frame` every MOCK_TICK_MS (200ms → 5 fps), so one frame is
// 6 engine ticks at CE's 30Hz. Everything below is expressed in frames and
// converted on the way out.

/** Engine ticks per mock frame (30Hz engine ÷ 5fps feed). */
const TICKS_PER_FRAME = 6;
/** Frames between kill-script steps — 2.4s, a believable Slayer cadence. */
const STEP_FRAMES = 12;
/** Frames a victim stays dead — 3s, so the ring has time to visibly drain. */
const DEAD_FRAMES = 15;
/** Match length in script steps: three passes over the 17-step loop. */
const MATCH_STEPS = 51;
const MATCH_FRAMES = MATCH_STEPS * STEP_FRAMES; // 612 frames ≈ 122s
/** Dead air after the match — long enough to read the out-animations. */
const IDLE_FRAMES = 25; // 5s
const CYCLE_FRAMES = MATCH_FRAMES + IDLE_FRAMES;

/** Fixed match-start instant so event timestamps advance without a clock read
 * (keeps every export deterministic for a given frame). */
const MATCH_START_MS = Date.parse('2026-06-20T02:00:00Z');

interface MockSeed {
	index: number;
	name: string;
	team: number;
	/** Kills at the opening whistle. Per team these MUST sum to the team's
	 * starting score — the postgame ledger prints the team aggregate directly
	 * above rows that sum to this, so a mismatch reads as a bug. */
	kills: number;
	deaths: number;
	assists: number;
	isLocal: boolean;
	/** CE/H2 armor-palette index (0..17) — tints the player card's Spartan. Warm
	 * indices for the red team, cool for blue, so the roster reads team-wise while
	 * every card still shows a distinct hue (real team games lock armor to the
	 * team colour; this is preview data chosen to exercise the palette). */
	armorColor: number;
	/** base health/shields as a 0..1 fraction; the frame counter wobbles it. */
	baseHealth: number;
	baseShields: number;
}

// Red opens at 26, blue at 31 — blue leads by 5. The script hands red a net +3
// every loop, so red takes the lead ONCE, around 70s in, and holds it. Red
// finishes on exactly 50 (the score limit), which is what ends the match.
//
// The deficit is 5 rather than 4 on purpose. Red and blue kills alternate for
// the first eleven steps of each loop, so the gap oscillates by 1 as it climbs;
// a 4-point deficit put that oscillation right on top of the tie, and the
// scorebug's leader-orange blinked off and on six times over half a minute
// before the real handover. A 5-point deficit lands the crossing in the loop's
// monotone tail instead, so the lead changes hands once through a single brief
// level scoreline.
const SEEDS: MockSeed[] = [
	{
		index: 0,
		name: 'Stewball',
		team: 0,
		kills: 8,
		deaths: 4,
		assists: 4,
		isLocal: true,
		armorColor: 2, // Red
		baseHealth: 0.82,
		baseShields: 1
	},
	{
		index: 1,
		name: 'gravemind',
		team: 0,
		kills: 7,
		deaths: 9,
		assists: 6,
		isLocal: true,
		armorColor: 11, // Orange
		baseHealth: 0.55,
		baseShields: 0.4
	},
	{
		index: 2,
		name: 'noble_six',
		team: 0,
		kills: 6,
		deaths: 5,
		assists: 8,
		isLocal: false,
		armorColor: 16, // Maroon
		baseHealth: 1,
		baseShields: 0.75
	},
	{
		index: 3,
		name: 'CmdrKeyes',
		team: 0,
		kills: 5,
		deaths: 6,
		assists: 3,
		isLocal: false,
		armorColor: 17, // Salmon
		baseHealth: 0.7,
		baseShields: 0.5
	},
	{
		index: 4,
		name: 'TheArbiter',
		team: 1,
		kills: 10,
		deaths: 5,
		assists: 5,
		isLocal: false,
		armorColor: 3, // Blue
		baseHealth: 0.9,
		baseShields: 0.9
	},
	{
		index: 5,
		name: 'TartarusX',
		team: 1,
		kills: 8,
		deaths: 8,
		assists: 2,
		isLocal: false,
		armorColor: 10, // Cobalt
		baseHealth: 0.3,
		baseShields: 0
	},
	{
		index: 6,
		name: 'Regret',
		team: 1,
		kills: 7,
		deaths: 8,
		assists: 7,
		isLocal: false,
		armorColor: 9, // Cyan
		baseHealth: 0.65,
		baseShields: 0.6
	},
	{
		index: 7,
		name: 'flood_carrier',
		team: 1,
		kills: 6,
		deaths: 7,
		assists: 1,
		isLocal: false,
		armorColor: 12, // Teal
		baseHealth: 1,
		baseShields: 1
	}
];

const SCORE_LIMIT = 50;

function clamp01(n: number): number {
	return n < 0 ? 0 : n > 1 ? 1 : n;
}

// ---------------------------------------------------------------------------
// The kill script
// ---------------------------------------------------------------------------

interface KillStep {
	/** null = no attributed killer (suicide / fall / environment / unknown). */
	killer: number | null;
	victim: number;
	weapon: string;
	cause: DeathCause;
}

// One loop of the scripted exchange. Three properties are load-bearing and any
// edit has to preserve them (the unit tests assert all three):
//
//  1. RED NETS EXACTLY +3. Eight red kills, five blue, four unattributed. With
//     a 4-point opening deficit that puts the lead change in loop 2.
//  2. THE RUNNING DIFFERENTIAL NEVER SWINGS MORE THAN 1. Red and blue kills
//     alternate strictly until red's surplus is banked, so the score gap
//     crosses zero exactly once instead of ping-ponging around the crossing
//     point for half a minute.
//  3. NOBODY KILLS WHILE DEAD. A victim is out for DEAD_FRAMES (15) which
//     overlaps the next step (12 frames), so no player may appear as killer in
//     the step immediately after they were a victim.
//
// It also exercises all six DeathCause values, every one of which is a distinct
// render branch in the kill feed and the respawn ring's plate.
const KILL_SCRIPT: KillStep[] = [
	// R — Stewball opens; his streak is the spree-badge demo.
	{ killer: 0, victim: 5, weapon: 'weapons\\pistol\\pistol', cause: 'kill' },
	// B — gravemind's first death of the loop, with a named killer: this is the
	// KILLED BY plate on the POV overlay's respawn ring.
	{ killer: 4, victim: 1, weapon: 'weapons\\sniper rifle\\sniper rifle', cause: 'kill' },
	// R — Stewball's second in a row (badge goes orange at 3 next loop).
	{ killer: 0, victim: 4, weapon: 'weapons\\assault rifle\\assault rifle', cause: 'kill' },
	// B — and the streak breaks; the badge cascades.
	{ killer: 6, victim: 0, weapon: 'weapons\\plasma rifle\\plasma rifle', cause: 'kill' },
	{ killer: 2, victim: 6, weapon: 'weapons\\pistol\\pistol', cause: 'kill' },
	{ killer: 4, victim: 2, weapon: 'weapons\\sniper rifle\\sniper rifle', cause: 'kill' },
	{ killer: 3, victim: 7, weapon: 'weapons\\needler\\needler', cause: 'kill' },
	// B — betrayal (blue on blue), so the shame columns and the team_kill flag
	// both have something to render.
	{ killer: 5, victim: 6, weapon: 'weapons\\shotgun\\shotgun', cause: 'betrayal' },
	{ killer: 1, victim: 4, weapon: 'weapons\\rocket launcher\\rocket launcher', cause: 'kill' },
	{ killer: 7, victim: 1, weapon: 'weapons\\assault rifle\\assault rifle', cause: 'kill' },
	{ killer: 0, victim: 5, weapon: 'weapons\\sniper rifle\\sniper rifle', cause: 'kill' },
	// N — gravemind's third death, unattributed: the RESPAWNING pill (no plate).
	{ killer: null, victim: 1, weapon: 'weapons\\frag grenade\\frag grenade', cause: 'suicide' },
	{ killer: 2, victim: 7, weapon: 'weapons\\plasma rifle\\plasma rifle', cause: 'kill' },
	{ killer: null, victim: 3, weapon: '', cause: 'fall' },
	{ killer: 2, victim: 4, weapon: 'weapons\\assault rifle\\assault rifle', cause: 'kill' },
	{ killer: null, victim: 5, weapon: '', cause: 'environment' },
	{ killer: null, victim: 6, weapon: '', cause: 'unknown' }
];

const LOOP_STEPS = KILL_SCRIPT.length; // 17

// Power-up windows, in frames within one script loop. Both sit on Stewball —
// he's a local, so they land on the POV bar where the effects actually read.
// Chosen to avoid his death window (frames 36–51) so the card isn't cloaked or
// overshielded while it's showing a respawn ring.
// The camo window is long (19s of the nominal 30s decay) so the wipe travels
// far enough to cross the ghost threshold at 50% — both sides of that branch
// get previewed instead of only the fully-ghosted one.
const CAMO_WINDOW: [number, number] = [56, 152];
const OVERSHIELD_WINDOW: [number, number] = [156, 200];
const POWERUP_SEAT = 0;

// ---------------------------------------------------------------------------
// Forward simulation
// ---------------------------------------------------------------------------

interface MockPlayerState {
	kills: number;
	deaths: number;
	suicides: number;
	betrayals: number;
	streak: number;
	bestStreak: number;
	/** Frame of this player's most recent death; null if they haven't died. */
	deathFrame: number | null;
}

interface MockMatch {
	/** 0-based frame within the cycle. */
	cycleFrame: number;
	/** Frame within the match; frozen at the last match frame during idle. */
	matchFrame: number;
	/** Frame within the current script loop — drives the power-up windows. */
	loopFrame: number;
	phase: 'live' | 'idle';
	players: MockPlayerState[];
	teamScores: [number, number];
	/** Newest-first, exactly like the live WS store's per-instance event log. */
	events: DeathEvent[];
}

function blankState(): MockPlayerState[] {
	return SEEDS.map(() => ({
		kills: 0,
		deaths: 0,
		suicides: 0,
		betrayals: 0,
		streak: 0,
		bestStreak: 0,
		deathFrame: null
	}));
}

/** Single-entry memo: mockGame / mockTick / mockEvents are all called for the
 * same frame on every feed update, and each would otherwise re-run the whole
 * forward simulation. */
let memoFrame = -1;
let memoMatch: MockMatch | null = null;

function derive(frame: number): MockMatch {
	if (memoMatch && memoFrame === frame) return memoMatch;
	const out = simulate(frame);
	memoFrame = frame;
	memoMatch = out;
	return out;
}

function simulate(frame: number): MockMatch {
	const f = Math.max(0, Math.floor(frame));
	const cycleFrame = f % CYCLE_FRAMES;
	const live = cycleFrame < MATCH_FRAMES;
	// During the idle gap the match state freezes on its final frame, so the
	// postgame ledger and the scorebug's latched clock both keep reading the
	// finished match rather than snapping back to zero.
	const matchFrame = live ? cycleFrame : MATCH_FRAMES - 1;

	const players = blankState();
	const events: DeathEvent[] = [];
	// A step lands ON its frame, so the step at matchFrame itself has happened.
	const stepsDone = Math.min(Math.floor(matchFrame / STEP_FRAMES) + 1, MATCH_STEPS);

	for (let step = 0; step < stepsDone; step++) {
		const s = KILL_SCRIPT[step % LOOP_STEPS];
		const at = step * STEP_FRAMES;
		const victim = players[s.victim];

		victim.deaths += 1;
		victim.streak = 0;
		victim.deathFrame = at;
		if (s.cause === 'suicide') victim.suicides += 1;

		if (s.killer != null) {
			const killer = players[s.killer];
			killer.kills += 1;
			killer.streak += 1;
			if (killer.streak > killer.bestStreak) killer.bestStreak = killer.streak;
			if (s.cause === 'betrayal') killer.betrayals += 1;
		}

		events.unshift(buildDeath(step, s));
	}

	const teamScores: [number, number] = [0, 0];
	SEEDS.forEach((seed, i) => {
		teamScores[seed.team === 1 ? 1 : 0] += seed.kills + players[i].kills;
	});

	return {
		cycleFrame,
		matchFrame,
		loopFrame: matchFrame % (LOOP_STEPS * STEP_FRAMES),
		phase: live ? 'live' : 'idle',
		players,
		teamScores,
		events: events.slice(0, 12)
	};
}

function buildDeath(step: number, s: KillStep): DeathEvent {
	const tick = step * STEP_FRAMES * TICKS_PER_FRAME;
	return {
		seq: step + 1,
		tick,
		at: new Date(MATCH_START_MS + (tick / 30) * 1000).toISOString(),
		event_type: 'death',
		victim: seedRef(s.victim),
		victim_pos: { x: 0, y: 0, z: 0 },
		killer: s.killer == null ? null : seedRef(s.killer),
		killer_pos: s.killer == null ? null : { x: 0, y: 0, z: 0 },
		cause: s.cause,
		weapon: s.weapon,
		team_kill: s.cause === 'betrayal',
		respawn_in_ticks: DEAD_FRAMES * TICKS_PER_FRAME
	};
}

function seedRef(i: number): PlayerRef {
	const s = SEEDS[i];
	return { index: s.index, name: s.name, team: s.team, armor_color: s.armorColor };
}

/** Frames this player has been dead, or null if they're alive right now. */
function deadFor(m: MockMatch, i: number): number | null {
	if (m.phase !== 'live') return null; // match over — everyone is back up
	const at = m.players[i].deathFrame;
	if (at == null) return null;
	const elapsed = m.matchFrame - at;
	return elapsed >= 0 && elapsed < DEAD_FRAMES ? elapsed : null;
}

function inWindow(loopFrame: number, [from, to]: [number, number]): boolean {
	return loopFrame >= from && loopFrame < to;
}

// ---------------------------------------------------------------------------
// Positions (for the top-down visualizer; the OBS overlays ignore these)
// ---------------------------------------------------------------------------

// Believable Bloodgulch-ish layout so the top-down VISUALIZER has something to
// render in mock mode. The OBS scoreboard/status overlays ignore positions +
// the objects/scenario-spawn layers, so populating them here is additive. Red
// base sits NW, Blue base SE; players 0 and 4 roam toward mid-map so their dots
// visibly move — the same movement that validates the position offsets live.
// Real Blood Gulch coordinates: the extracted structure-BSP mesh spans
// x[6,132] y[-190,-45] z[-0.3,26], so the mock markers sit INSIDE the real map in
// the 3D visualizer (and the 2D map auto-fits the same layout). Red base near the
// canyon's north mouth, Blue near the south, MID between them; z ~ floor height.
const RED_BASE = { x: 60, y: -70 };
const BLUE_BASE = { x: 70, y: -165 };
const MID = { x: 65, y: -118 };
const FLOOR_Z = 2;

function mockPlayerPos(
	s: MockSeed,
	frame: number,
	dead: boolean
): { x: number; y: number; z: number } {
	const base = s.team === 0 ? RED_BASE : BLUE_BASE;
	if (dead) return { x: base.x, y: base.y, z: FLOOR_Z }; // respawning — sit at base
	if (s.index === 0 || s.index === 4) {
		// Roamer: ping-pong between base and mid-map.
		const t = (Math.sin(frame / 30 + s.index) + 1) / 2;
		return {
			x: base.x + (MID.x - base.x) * t,
			y: base.y + (MID.y - base.y) * t,
			z: FLOOR_Z + 4 * t
		};
	}
	// Orbit near the base so each dot drifts a little.
	const a = frame / 40 + s.index;
	const r = 8 + (s.index % 3) * 4;
	return {
		x: base.x + Math.cos(a) * r,
		y: base.y + Math.sin(a) * r,
		z: FLOOR_Z + (s.index % 2) * 3
	};
}

function mockPlayerAim(s: MockSeed): { x: number; y: number; z: number } {
	// Face the enemy base.
	const target = s.team === 0 ? BLUE_BASE : RED_BASE;
	const base = s.team === 0 ? RED_BASE : BLUE_BASE;
	const dx = target.x - base.x;
	const dy = target.y - base.y;
	const len = Math.hypot(dx, dy) || 1;
	return { x: dx / len, y: dy / len, z: 0 };
}

// ---------------------------------------------------------------------------
// Exports
// ---------------------------------------------------------------------------

export function mockGame(frame = 0): GamePayload {
	const m = derive(frame);
	return {
		phase: m.phase,
		started_at: new Date(MATCH_START_MS).toISOString(),
		last_read_at: new Date(
			MATCH_START_MS + ((m.matchFrame * TICKS_PER_FRAME) / 30) * 1000
		).toISOString(),
		engine_tick: m.matchFrame * TICKS_PER_FRAME,
		iterations: 1,
		config: {
			gametype: 'slayer',
			variant_name: 'Team Slayer',
			is_team_game: true,
			score_limit: SCORE_LIMIT,
			time_limit_ticks: 0
		},
		team_scores: [
			{ team: 0, score: m.teamScores[0] },
			{ team: 1, score: m.teamScores[1] }
		],
		players: SEEDS.map((s, i) => {
			const st = m.players[i];
			const kills = s.kills + st.kills;
			return {
				index: s.index,
				name: s.name,
				team: s.team,
				armor_color: s.armorColor,
				// score IS kills in Slayer, and per team these sum to team_scores —
				// the postgame ledger prints both, so they have to agree.
				score: kills,
				kills,
				deaths: s.deaths + st.deaths,
				assists: s.assists,
				ctf_score: 0,
				team_kills: st.betrayals,
				suicides: st.suicides,
				kill_streak: st.streak,
				multikill: 0,
				shots_fired: 0,
				shots_hit: 0,
				// Accumulated match stats (acc_* — the server-side HaloCaster-port
				// deltas), scaled off live kills/deaths so every postgame column +
				// footer total climbs through the match instead of sitting still.
				acc_shots_fired: 40 + kills * 9 + s.index * 7,
				acc_grenade_throws: 3 + (s.index % 3) * 2 + Math.floor(st.kills / 2),
				acc_melees: 1 + (s.index % 2) * 2 + Math.floor(st.kills / 3),
				acc_damage_dealt: 300 + kills * 82,
				acc_damage_received: 250 + (s.deaths + st.deaths) * 78,
				acc_camo_pickups: s.index % 2,
				acc_overshield_pickups: (s.index + 1) % 2,
				best_kill_streak: Math.max(2 + (s.kills % 3), st.bestStreak),
				is_local: s.isLocal,
				local_index: s.isLocal ? s.index : null,
				machine_index: s.team,
				controller_index: s.isLocal ? s.index : null
			};
		}),
		machines: [
			{ index: 0, name: 'RED-XBOX', is_local: true },
			{ index: 1, name: 'BLU-XBOX', is_local: false }
		],
		network: null
	};
}

function mockTickPlayer(
	s: MockSeed,
	m: MockMatch,
	frame: number
): TickPayloadV2['players'][number] {
	const dead = deadFor(m, s.index);
	const alive = dead === null;
	// Per-player phase offset so the bars don't all pulse in lockstep.
	const wobble = Math.sin((frame + s.index * 3) / 6) * 0.12;
	const health = alive ? clamp01(s.baseHealth + wobble) : 0;

	const camo = alive && s.index === POWERUP_SEAT && inWindow(m.loopFrame, CAMO_WINDOW);
	const os = alive && s.index === POWERUP_SEAT && inWindow(m.loopFrame, OVERSHIELD_WINDOW);
	// Overshield reads as shields ABOVE 1 (the card's conic rings only render
	// past 1); drain it back down to 1 across the window so the rings visibly
	// deplete instead of holding a constant arc.
	const osProgress = os
		? (m.loopFrame - OVERSHIELD_WINDOW[0]) / (OVERSHIELD_WINDOW[1] - OVERSHIELD_WINDOW[0])
		: 0;
	const shields = !alive ? 0 : os ? 2 - osProgress : clamp01(s.baseShields + wobble);

	return {
		index: s.index,
		alive,
		// Drains one whole step per frame down to a final 6 — the ring sweeps off
		// the unrounded seconds, so it has to move every frame, not every second.
		respawn_in_ticks: alive ? null : (DEAD_FRAMES - (dead as number)) * TICKS_PER_FRAME,
		pos: mockPlayerPos(s, frame, !alive),
		vel: { x: 0, y: 0, z: 0 },
		aim: mockPlayerAim(s),
		zoom_level: 0,
		crouch_scale: 0,
		// Emit the engine's 0..1 fraction, matching the live wire (health/shields
		// at 0x90/0x94 are RUNTIME-VERIFIED 0..1, not 0..75) — except during the
		// overshield window, where the overlay's own model is 0..2. Max* are the
		// absolute ceilings.
		health,
		shields,
		max_health: 75,
		max_shields: 75,
		has_camo: camo,
		has_overshield: os,
		frags: 2,
		plasmas: 1,
		selected_weapon_slot: 0,
		biped_tag: 'characters\\cyborg\\cyborg',
		actions: {
			crouching: false,
			jumping: false,
			firing: alive && frame % 20 < 6 && s.index % 2 === 0,
			flashlight: false,
			throwing_grenade: false,
			meleeing: false,
			using: false
		},
		weapons: [
			{
				slot: 0,
				tag:
					s.index % 2 === 0 ? 'weapons\\assault rifle\\assault rifle' : 'weapons\\pistol\\pistol',
				object_id: 1000 + s.index,
				ammo_mag: 32 - (frame % 32),
				ammo_pack: 96,
				charge: null,
				heat: 0,
				reloading: false
			}
		]
	};
}

export function mockTick(frame = 0): TickPayloadV2 {
	const m = derive(frame);
	return {
		players: SEEDS.map((s) => mockTickPlayer(s, m, frame)),
		// Tracked power items: rockets sit mid-map, the sniper is held by Arbiter
		// (4), the overshield is respawning (counted, off-map), camo is on the
		// floor. spawn_ids line up with mockPowerItemSpawns() for labels.
		power_items: [
			{
				spawn_id: 0,
				status: 'world',
				held_by: null,
				pos: { x: 65, y: -118, z: 2 },
				respawn_in_ticks: null
			},
			{ spawn_id: 1, status: 'held', held_by: 4, pos: null, respawn_in_ticks: null },
			{
				spawn_id: 2,
				status: 'respawning',
				held_by: null,
				pos: null,
				respawn_in_ticks: 300 - (frame % 300)
			},
			{
				spawn_id: 3,
				status: 'world',
				held_by: null,
				pos: { x: 52, y: -148, z: 2 },
				respawn_in_ticks: null
			}
		],
		// Slayer has no flags; the visualizer renders the flag layer live on CTF.
		ctf_flags: [],
		game_globals: {
			map_loaded: 1,
			active: m.phase === 'live' ? 1 : 0,
			game_loading_in_progress: 0,
			precache_map_status: 1,
			stored_global_random: 0
		},
		locals: []
	};
}

/** Recent deaths, NEWEST-FIRST — exactly how the live WS store keeps the
 * per-instance event log, so the overlays render identically from either
 * source. The respawn ring's KILLED BY plate reads the newest death per victim
 * out of this list. */
export function mockEvents(frame = 0): AnyEvent[] {
	return derive(frame).events;
}

export function mockScenario(): ScenarioPayload {
	return {
		map: 'levels\\test\\bloodgulch\\bloodgulch',
		game_difficulty: 2,
		fog: null,
		memory_regions: null,
		object_types: [],
		player_spawns: mockPlayerSpawns(),
		power_item_spawns: mockPowerItemSpawns(),
		tag_defs: {}
	};
}

function mockPlayerSpawns(): ScenarioPayload['player_spawns'] {
	const ring = (base: { x: number; y: number }, team: number, start: number) =>
		[0, 1, 2, 3].map((i) => ({
			index: start + i,
			pos: {
				x: base.x + Math.cos((i / 4) * Math.PI * 2) * 10,
				y: base.y + Math.sin((i / 4) * Math.PI * 2) * 10,
				z: 0
			},
			facing: 0,
			team_index: team,
			bsp_index: 0,
			gametypes: []
		}));
	return [...ring(RED_BASE, 0, 0), ...ring(BLUE_BASE, 1, 4)];
}

function mockPowerItemSpawns(): ScenarioPayload['power_item_spawns'] {
	return [
		{
			spawn_id: 0,
			tag: 'weapons\\rocket launcher\\rocket launcher',
			interval_ticks: 3600,
			gametype_mask: 0,
			pos: { x: 65, y: -118, z: 2 }
		},
		{
			spawn_id: 1,
			tag: 'weapons\\sniper rifle\\sniper rifle',
			interval_ticks: 3600,
			gametype_mask: 0,
			pos: { x: 95, y: -100, z: 5 }
		},
		{
			spawn_id: 2,
			tag: 'powerups\\over shield\\over shield',
			interval_ticks: 5400,
			gametype_mask: 0,
			pos: { x: 45, y: -135, z: 5 }
		},
		{
			spawn_id: 3,
			tag: 'powerups\\active camouflage\\active camouflage',
			interval_ticks: 5400,
			gametype_mask: 0,
			pos: { x: 52, y: -148, z: 2 }
		},
		{
			spawn_id: 4,
			tag: 'weapons\\plasma rifle\\plasma rifle',
			interval_ticks: 1800,
			gametype_mask: 0,
			pos: { x: 80, y: -95, z: 1 }
		}
	];
}

export function mockObjects(): ObjectsPayload {
	const veh = (id: number, tag: string, x: number, y: number, owner: number | null) => ({
		object_id: id,
		tag,
		type: 1,
		flags: 0,
		pos: { x, y, z: 0 },
		ang_vel: { x: 0, y: 0, z: 0 },
		time_existing: 600,
		owner_unit: owner,
		owner_object: null,
		ultimate_parent: id
	});
	return {
		objects: [
			veh(5001, 'vehicles\\warthog\\warthog', 55, -78, null),
			veh(5002, 'vehicles\\ghost\\ghost', 82, -158, 5), // occupied
			veh(5003, 'vehicles\\banshee\\banshee', 95, -150, null),
			{
				object_id: 5100,
				tag: 'weapons\\assault rifle\\assault rifle',
				type: 2,
				flags: 0,
				pos: { x: 68, y: -110, z: 0 },
				ang_vel: { x: 0, y: 0, z: 0 },
				time_existing: 120,
				owner_unit: null,
				owner_object: null,
				ultimate_parent: 5100
			}
		],
		projectiles: [
			{
				object_id: 6001,
				tag: 'weapons\\frag grenade\\frag grenade',
				pos: { x: 60, y: -115, z: 2 },
				flags: 0,
				action: 0,
				detonation_timer: 1.2,
				distance_traveled: 4
			}
		]
	};
}

// H2 emblem/appearance for the Halo 2-THEMED broadcast previews. Halo 2 players
// carry an emblem (foreground symbol + background plate + four colours) the way
// CE players never did — the H2 broadcast theme renders it on the Spartan's
// chest and as a card badge. Live H2 will source this from the (unfinished) H2
// scraper's profile read; until then, mockAppearance derives a stable,
// distinct-per-slot emblem so the H2 theme is previewable. Deterministic on the
// slot index so a given card looks the same every frame.
//
// NOTE (CE vs H2): CE has NO emblem system — the CE theme intentionally never
// calls this. This is Halo 2 broadcast preview data only.
export function mockAppearance(index: number): Appearance {
	const seed = SEEDS[((index % SEEDS.length) + SEEDS.length) % SEEDS.length];
	const armorPrimary = seed.armorColor % 18;
	return {
		[H2_KEYS.armorPrimary]: armorPrimary,
		[H2_KEYS.armorSecondary]: (armorPrimary + 9) % 18,
		[H2_KEYS.emblemPrimary]: 0, // white symbol
		[H2_KEYS.emblemSecondary]: (armorPrimary + 4) % 18,
		// The Arbiter (slot 4) fittingly rides in as an Elite; everyone else is a
		// Spartan (Master Chief bust).
		[H2_KEYS.character]: index === 4 ? 3 : 0,
		[H2_KEYS.foreground]: (index * 7 + 12) % 64,
		[H2_KEYS.background]: (index * 5 + 3) % 32,
		[H2_KEYS.flags]: 0
	};
}

// Mock stand-in for a user's PocketBase avatar file (users.avatar): a small
// self-contained SVG data-URI — coloured plate + the gamertag's initial — so the
// preview exercises the cards' PB-avatar <img> spot with a real image and no
// backend. Live mode replaces this with the /api/files/users/... thumb URL.
function mockAvatarDataURI(name: string, hex: string): string {
	const initial = (name[0] ?? '?').toUpperCase();
	const svg =
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">` +
		`<rect width="64" height="64" rx="10" fill="${hex}"/>` +
		`<rect width="64" height="32" rx="10" fill="rgba(255,255,255,0.14)"/>` +
		`<text x="32" y="43" text-anchor="middle" font-family="sans-serif" font-size="32" font-weight="700" fill="#fff">${initial}</text>` +
		`</svg>`;
	return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

// Stand-in for a picked nameplate's 600×100 banner art (users.nameplate → the
// `plate` field). Only ONE seed carries it, so a preview shows the banner
// treatment and the plain navy pill side by side.
function mockPlateDataURI(hex: string): string {
	const svg =
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 100">` +
		`<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">` +
		`<stop offset="0" stop-color="${hex}"/><stop offset="1" stop-color="#0b1220"/>` +
		`</linearGradient></defs>` +
		`<rect width="600" height="100" fill="url(#g)"/>` +
		`<path d="M0 100 L180 0 L240 0 L60 100 Z" fill="rgba(255,255,255,0.10)"/>` +
		`<path d="M120 100 L300 0 L330 0 L150 100 Z" fill="rgba(255,255,255,0.06)"/>` +
		`</svg>`;
	return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

// Mock identity table — the ?mock=1 stand-in for the live /api/public/profiles
// endpoint, keyed by lowercased scraped name exactly as the endpoint keys its
// reply. Each seed gets a display name (their "default gamertag"), a PB avatar
// image (data-URI stand-in), a CE armor colour and an H2 emblem/appearance.
//
// FRAME-INDEPENDENT ON PURPOSE: the profile store snapshots this exactly once
// behind a `mockLoaded` latch, so anything derived from the frame counter here
// would silently freeze at frame 0.
//
// Deliberate gaps so the preview exercises every fallback branch:
//   - SEED 5 (TartarusX): NOT identified at all → trimmed scraped name +
//     placeholder emblem.
//   - SEED 7 (flood_carrier): identified, but NO avatar image → name swaps,
//     avatar spot falls back to the placeholder.
export function mockProfiles(): Record<
	string,
	{
		display?: string;
		avatar?: string;
		motto?: string;
		plate?: string;
		ce?: { color: number };
		h2?: { appearance: Appearance };
	}
> {
	const out: Record<
		string,
		{
			display?: string;
			avatar?: string;
			motto?: string;
			plate?: string;
			ce?: { color: number };
			h2?: { appearance: Appearance };
		}
	> = {};
	for (const s of SEEDS) {
		if (s.index === 5) continue; // unidentified → full fallback demo
		out[s.name.toLowerCase()] = {
			// Stand-in for the user's default gamertag: visibly different from the
			// scraped name so the swap is obvious in a preview.
			display: `NC ${s.name}`,
			...(s.index === 7
				? {} // identified without an avatar image → avatar-spot fallback demo
				: { avatar: mockAvatarDataURI(s.name, colorHex(CE_COLORS, s.armorColor)) }),
			// One seed carries a motto + nameplate art so the plate's second line and
			// its banner treatment both preview under ?mock=1 (settings Stream tab
			// data on the wire); the rest stay on the plain pill for comparison.
			...(s.index === 0
				? {
						motto: 'No tunnel is safe from me',
						plate: mockPlateDataURI(colorHex(CE_COLORS, s.armorColor))
					}
				: {}),
			ce: { color: s.armorColor },
			h2: { appearance: mockAppearance(s.index) }
		};
	}
	return out;
}
