// View-model for the overview tab. Sources the v2 store (per-class
// payloads from PR 20) and projects into the v1 shapes the section
// components still consume:
//   - gameData ← v2ToV1GameData(game, scenario)
//   - tick     ← v2ToV1Tick(tick, objects, game)
//   - events   ← v2 AnyEvent[] wrapped in v1 Envelope shape
//   - previousGame ← v2ToV1PreviousGame(previous_game)
//
// Plain-TS so reactivity is established at the call site by wrapping
// in $derived.by().

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { DebugContext } from '../context';
import type {
	Envelope,
	GameData,
	GamePlayer,
	Phase,
	PreviousGameInfo,
	StateInputs,
	TeamScore,
	TickCTFFlag,
	TickNetMachine,
	TickNetPlayer,
	TickNetwork,
	TickObject,
	TickPayload,
	TickPlayer,
	TickProjectile,
	WeaponInfo,
	XYZ
} from '$lib/types/scraper';
import type {
	AnyEvent,
	GameNetwork,
	GamePayload,
	ObjectsPayload,
	TickPayloadV2,
	TickPlayerV2,
	TickWeaponV2
} from '$lib/types/scraper-v2';
import { v2ToV1GameData } from '../shared/v2-to-v1-game';
import { v2ToV1PreviousGame } from '../shared/previous-game-adapter';
import { teamAccent, teamLabel } from '../shared/util';

// Tick v2→v1 projection — kept colocated with the overview VM (the only
// remaining caller). The Tick tab itself reads v2 directly now, so this
// adapter exists purely to feed the legacy v1-typed Overview sections
// (the OverviewCard tick block + InspectStateInputsSection) until they
// migrate. Drop alongside the v1 type cleanup.

function vec3ToXYZ(v: { x: number; y: number; z: number } | null | undefined): XYZ | null {
	if (!v) return null;
	return { x: v.x, y: v.y, z: v.z };
}

function v2WeaponToV1(w: TickWeaponV2): WeaponInfo {
	return {
		slot: w.slot,
		object_id: w.object_id,
		tag: w.tag,
		ammo_pack: w.ammo_pack,
		ammo_mag: w.ammo_mag,
		charge: w.charge,
		is_energy: w.charge !== null && w.charge !== undefined,
		extended: {
			heat_meter: w.heat,
			used_energy: 0,
			owner_handle: 0,
			is_reloading: w.reloading ? 1 : 0,
			reload_time: 0,
			can_fire: 0,
			world: null
		},
		tag_data: null
	};
}

function v2PlayerToV1(p: TickPlayerV2): TickPlayer {
	return {
		index: p.index,
		alive: p.alive,
		respawn_in_ticks: p.respawn_in_ticks,
		x: p.pos.x,
		y: p.pos.y,
		z: p.pos.z,
		vx: p.vel.x,
		vy: p.vel.y,
		vz: p.vel.z,
		aim_x: p.aim.x,
		aim_y: p.aim.y,
		aim_z: p.aim.z,
		zoom_level: p.zoom_level,
		crouchscale: p.crouch_scale,
		health: p.health,
		shields: p.shields,
		has_camo: p.has_camo,
		camo_timer: null,
		has_overshield: p.has_overshield,
		frags: p.frags,
		plasmas: p.plasmas,
		selected_weapon_slot: p.selected_weapon_slot,
		is_crouching: p.actions.crouching,
		is_jumping: p.actions.jumping,
		is_firing: p.actions.firing,
		is_shooting: p.actions.firing,
		is_flashlight_on: p.actions.flashlight,
		is_throwing_grenade: p.actions.throwing_grenade,
		is_meleeing: p.actions.meleeing,
		is_pressing_action: p.actions.using,
		is_holding_action: p.actions.using,
		weapons: p.weapons?.map(v2WeaponToV1) ?? null,
		extended: null,
		bones: null,
		update_queue: null,
		biped_tag: null
	};
}

function v2NetworkToV1(n: GameNetwork | null | undefined): TickNetwork | null {
	if (!n) return null;
	const machines: TickNetMachine[] = [];
	const network_players: TickNetPlayer[] = [];
	return {
		client: n.client
			? {
					machine_index: n.client.machine_index,
					ping_target_ip: 0,
					packets_sent: n.client.packets_sent,
					packets_received: 0,
					average_ping: n.client.average_ping,
					ping_active: 0,
					seconds_to_game_start: n.countdown?.seconds_to_start ?? 0
				}
			: null,
		server: n.countdown
			? {
					countdown_active: n.countdown.active ? 1 : 0,
					countdown_paused: n.countdown.paused ? 1 : 0,
					countdown_adjusted_time: n.countdown.seconds_to_start
				}
			: null,
		game_data: null,
		machines,
		network_players
	};
}

function v2ObjectToV1(o: ObjectsPayload['objects'][number]): TickObject {
	return {
		object_id: o.object_id,
		tag: o.tag,
		type: o.type,
		flags: o.flags,
		x: o.pos.x,
		y: o.pos.y,
		z: o.pos.z,
		ang_vel_x: o.ang_vel.x,
		ang_vel_y: o.ang_vel.y,
		ang_vel_z: o.ang_vel.z,
		unk_damage_1: 0,
		time_existing: o.time_existing,
		owner_unit_ref: o.owner_unit ?? 0xffffffff,
		owner_object_ref: o.owner_object ?? 0xffffffff,
		ultimate_parent: o.ultimate_parent
	};
}

function v2ProjectileToV1(p: ObjectsPayload['projectiles'][number]): TickProjectile {
	return {
		object_id: p.object_id,
		tag: p.tag,
		x: p.pos.x,
		y: p.pos.y,
		z: p.pos.z,
		flags: p.flags,
		action: p.action,
		hit_material_type: 0,
		ignore_object_index: 0,
		detonation_timer: p.detonation_timer,
		detonation_timer_delta: 0,
		target_object_index: 0,
		arming_time_delta: 0,
		distance_traveled: p.distance_traveled,
		deceleration_timer: 0,
		deceleration_timer_delta: 0,
		deceleration: 0,
		maximum_damage_distance: 0,
		rotation_axis_x: 0,
		rotation_axis_y: 0,
		rotation_axis_z: 0,
		rotation_sine: 0,
		rotation_cosine: 0
	};
}

function v2PowerItemToV1(p: {
	spawn_id: number;
	status: string;
	held_by: number | null;
	pos: { x: number; y: number; z: number } | null;
	respawn_in_ticks: number | null;
}) {
	return {
		spawn_id: p.spawn_id,
		status: p.status as 'held' | 'world' | 'respawning',
		held_by: p.held_by,
		world_pos: vec3ToXYZ(p.pos),
		respawn_in_ticks: p.respawn_in_ticks
	};
}

function v2CTFToV1(f: {
	team: number;
	pos: { x: number; y: number; z: number };
	carrier: number | null;
	status: string;
}): TickCTFFlag {
	return {
		team: f.team,
		x: f.pos.x,
		y: f.pos.y,
		z: f.pos.z,
		carrier_index: f.carrier,
		status: f.status
	};
}

function v2ToV1Tick(
	tick: TickPayloadV2 | null,
	objects: ObjectsPayload | null,
	game: GamePayload | null
): TickPayload | null {
	if (!tick && !objects) return null;
	return {
		players: tick?.players?.map(v2PlayerToV1) ?? null,
		power_items: tick?.power_items?.map(v2PowerItemToV1) ?? null,
		game_globals: tick?.game_globals
			? {
					map_loaded: tick.game_globals.map_loaded,
					active: tick.game_globals.active,
					players_are_double_speed: 0,
					game_loading_in_progress: tick.game_globals.game_loading_in_progress,
					precache_map_status: tick.game_globals.precache_map_status,
					game_difficulty_level: 0,
					stored_global_random: tick.game_globals.stored_global_random
				}
			: null,
		player_count: tick?.players?.length ?? 0,
		local_count: tick?.locals?.length ?? 0,
		locals:
			tick?.locals?.map((l) => ({
				local_index: l.local_index,
				fp_weapon: l.fp_weapon
					? {
							weapon_rendered: 0,
							player_object: 0,
							weapon_object: 0,
							state: l.fp_weapon.state,
							idle_animation_threshold: 0,
							idle_animation_counter: 0,
							animation_id: l.fp_weapon.animation_id,
							animation_tick: l.fp_weapon.animation_tick
						}
					: null,
				observer_cam: l.observer_cam
					? {
							x: l.observer_cam.pos.x,
							y: l.observer_cam.pos.y,
							z: l.observer_cam.pos.z,
							vel_x: 0,
							vel_y: 0,
							vel_z: 0,
							aim_x: l.observer_cam.aim.x,
							aim_y: l.observer_cam.aim.y,
							aim_z: l.observer_cam.aim.z,
							fov: l.observer_cam.fov
						}
					: null,
				ias: null,
				gamepad: null,
				ui: null,
				player_control: l.player_control
					? {
							desired_yaw: l.player_control.desired_yaw,
							desired_pitch: l.player_control.desired_pitch,
							zoom_level: l.player_control.zoom_level,
							aim_assist_target: l.player_control.aim_assist_target ?? 0,
							aim_assist_near: 0,
							aim_assist_far: 0
						}
					: null,
				look_yaw_rate: 0,
				look_pitch_rate: 0
			})) ?? null,
		network: v2NetworkToV1(game?.network),
		data_queue: null,
		ctf_flags: tick?.ctf_flags?.map(v2CTFToV1) ?? null,
		objects: objects?.objects?.map(v2ObjectToV1) ?? null,
		projectiles: objects?.projectiles?.map(v2ProjectileToV1) ?? null
	};
}

type ScraperWSV2 = typeof scraperWSV2;

export type ScoreRow = {
	label: string;
	value: number;
	limit: number;
	accent: { bg: string; dot: string };
};

export type PlayerScoreTile = {
	name: string;
	score: number;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	rank: number;
};

export type OverviewVm = {
	gameData: GameData | null;
	tick: TickPayload | null;
	events: Envelope[];
	stateInputs: StateInputs | null;
	phase: Phase;
	previousGame: PreviousGameInfo | null;
	gameDataAtMs: number | undefined;
	tickAtMs: number | undefined;
	lastReadAtMs: number | undefined;
	previousGameEndedAtMs: number | undefined;
	tickValue: number | undefined;
	playerCount: number | undefined;
	scoreRows: ScoreRow[];
	playerScoreTiles: PlayerScoreTile[];
	previousMatchAutoOpen: boolean;
	lastReadStr: string;
	gameDataStr: string;
	tickStr: string;
	previousGameEndedStr: string;
	wsConnected: boolean;
	lastEventsReplyAtMs: number | undefined;
	lastEventsReplyStr: string;
	lastEventsReplyPhase: Phase | undefined;
};

function parseIsoMs(iso: string | undefined | null): number | undefined {
	if (!iso) return undefined;
	const t = Date.parse(iso);
	return Number.isFinite(t) ? t : undefined;
}

export function buildScoreRows(gameData: GameData | null): ScoreRow[] {
	if (!gameData || gameData.is_team_game !== true) return [];
	const limit = gameData.score_limit ?? 0;
	const teamScores: TeamScore[] = gameData.team_scores ?? [];
	return teamScores.map((ts) => ({
		label: teamLabel(ts.team),
		value: ts.score,
		limit,
		accent: teamAccent(ts.team)
	}));
}

// FFA: one mini-tile per player, sorted by score descending. Empty for team games.
export function buildPlayerScoreTiles(gameData: GameData | null): PlayerScoreTile[] {
	if (!gameData || gameData.is_team_game === true) return [];
	const players: GamePlayer[] = gameData.players ?? [];
	return [...players]
		.sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
		.map((p, i) => ({
			name: p.name || '—',
			score: p.score ?? 0,
			team: p.team,
			kills: p.kills ?? 0,
			deaths: p.deaths ?? 0,
			assists: p.assists ?? 0,
			rank: i + 1
		}));
}

// Wrap a v2 event payload as a v1 Envelope so RecentEvents/EventTile
// (already migrated in PR 25 to v2-aware summarizeEvent) keeps reading
// env.payload.event_type / env.tick.
function v2EventToV1(ev: AnyEvent, instance: string): Envelope {
	return {
		v: 2,
		type: 'event',
		instance,
		tick: ev.tick,
		payload: ev as unknown as Record<string, unknown>
	};
}

export function buildOverviewVm(name: string, ws: ScraperWSV2, ctx: DebugContext): OverviewVm {
	const v2Game = ws.game[name] ?? null;
	const v2Scenario = ws.scenario[name] ?? null;
	const v2Tick = ws.tick[name] ?? null;
	const v2Objects = ws.objects[name] ?? null;
	const v2Prev = ws.previousGame[name] ?? null;
	const v2Events = ws.events[name] ?? [];

	const projectedGame = v2ToV1GameData(v2Game, v2Scenario);
	const gameData = projectedGame ?? ctx.inspect?.game_data ?? null;

	const projectedTick = v2ToV1Tick(v2Tick, v2Objects, v2Game);
	const tick = projectedTick ?? ctx.inspect?.latest_tick ?? null;

	const events: Envelope[] =
		v2Events.length > 0
			? v2Events.map((ev) => v2EventToV1(ev, name))
			: (ctx.inspect?.recent_events ?? []);

	// state_inputs used to ride on the debug envelope; it moved to the
	// on-demand probe envelope so probe work doesn't run per-tick. The
	// overview tab can no longer surface it inline without requesting a
	// probe reply per render — fall back to inspect (always nil now,
	// since the v1 inspect endpoint also stopped populating probe
	// fields). The StateInputsGrid section will follow this VM to nil
	// and either render an empty state or be removed entirely in the
	// next overview-tab pass.
	const stateInputs: StateInputs | null = ctx.inspect?.state_inputs ?? null;

	const phase: Phase = ((v2Game?.phase ?? ctx.inspect?.phase) as Phase | undefined) ?? 'idle';

	const projectedPrev = v2ToV1PreviousGame(v2Prev);
	const previousGame = projectedPrev ?? ctx.inspect?.previous_game ?? null;

	const gameDataAtMs = ws.gameAt[name] ?? ws.scenarioAt[name] ?? ctx.inspectAt;
	const tickAtMs = ws.tickAt[name];
	const lastReadAtMs = parseIsoMs(v2Game?.last_read_at ?? ctx.inspect?.last_read_at);
	const previousGameEndedAtMs = parseIsoMs(previousGame?.ended_at);
	const tickValue = ws.engineTick[name] ?? ctx.inspect?.tick;
	const playerCount = gameData?.players?.length;

	return {
		gameData,
		tick,
		events,
		stateInputs,
		phase,
		previousGame,
		gameDataAtMs,
		tickAtMs,
		lastReadAtMs,
		previousGameEndedAtMs,
		tickValue,
		playerCount,
		scoreRows: buildScoreRows(gameData),
		playerScoreTiles: buildPlayerScoreTiles(gameData),
		// Auto-open the previous-match panel when there's no current match —
		// the most useful view when the runner is between matches.
		previousMatchAutoOpen: gameData == null,
		lastReadStr: ctx.relativeTime(lastReadAtMs),
		gameDataStr: ctx.relativeTime(gameDataAtMs),
		tickStr: ctx.relativeTime(tickAtMs),
		previousGameEndedStr: ctx.relativeTime(previousGameEndedAtMs),
		wsConnected: ws.connected,
		lastEventsReplyAtMs: ws.lastEventsReplyAt[name],
		lastEventsReplyStr: ctx.relativeTime(ws.lastEventsReplyAt[name]),
		lastEventsReplyPhase: ws.lastEventsReplyPhase[name] as Phase | undefined
	};
}
