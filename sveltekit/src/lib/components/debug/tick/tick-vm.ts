// View-model for the Tick tab. Sources data from scraperWSV2.tick[name]
// (v2 TickPayloadV2) + scraperWSV2.objects[name] (objects/projectiles
// moved to their own class) + scraperWSV2.game[name] (network moved
// onto the game class) and projects into the v1 TickPayload shape the
// existing section components still expect.
//
// Plain TS; reactivity is wired at the call site via $derived.by().
//
// Sub-section components (PlayerDetailPanel, NetworkSection, etc.) read
// v1-shaped types — when each migrates to v2 types directly, the
// adapter's job shrinks and eventually disappears in PR 26 cleanup.

import { scraperWSV2 } from '$lib/stores/scraper-ws-v2.svelte';
import type { DebugContext } from '../context';
import type {
	GameNetwork,
	GamePayload,
	ObjectsPayload,
	TickPayloadV2,
	TickPlayerV2,
	TickWeaponV2
} from '$lib/types/scraper-v2';
import type {
	GamePlayer,
	Phase,
	PowerItemStatus,
	TickCTFFlag,
	TickNetMachine,
	TickNetPlayer,
	TickNetwork,
	TickPayload,
	TickPlayer,
	TickObject,
	TickProjectile,
	WeaponInfo,
	XYZ
} from '$lib/types/scraper';
import type { KvField } from '../shared/KvGrid.svelte';

type ScraperWSV2 = typeof scraperWSV2;

// Render a plain Record into KvField[] for KvGrid. Annotation keys are
// derived from the prefix so every field in a sub-struct gets a stable
// pill identity (debug.tick.<section>.<field>).
export function recordToFields(
	value: object | null | undefined,
	annotationPrefix: string
): KvField[] {
	if (!value) return [];
	return Object.entries(value).map(([k, v]) => ({
		key: k,
		value: v,
		annotationKey: `${annotationPrefix}.${k}`
	}));
}

export type TickVm = {
	tick: TickPayload | null;
	phase: Phase;
	tickValue: number | undefined;
	tickStr: string;
	isTeamGame: boolean;
	playersByIndex: Map<number, GamePlayer>;
};

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
		// v2 collapses extended into top-level heat/reloading. Re-nest so
		// the v1 panel that reads weapon.extended.heat_meter still works.
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
		// v2 dropped camo_timer — debug class re-exposes the underlying
		// memory if needed; for now surface "absent" rather than fabricate.
		camo_timer: null,
		has_overshield: p.has_overshield,
		frags: p.frags,
		plasmas: p.plasmas,
		selected_weapon_slot: p.selected_weapon_slot,
		// v2 collapses the v1 9-bool soup. Map the overlapping pairs to
		// keep the v1 detail panel rendering — fine-grained distinctions
		// (is_shooting vs is_firing, is_pressing vs is_holding) are gone
		// from the wire so we report the merged truth on both sides.
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
		// extended/bones/update_queue moved to the debug class. The Tick
		// tab's detail panel reads these from the player struct; they'll
		// stay empty until the panel migrates to scraperWSV2.debug.
		extended: null,
		bones: null,
		update_queue: null,
		biped_tag: null
	};
}

function v2PowerItemToV1(p: {
	spawn_id: number;
	status: string;
	held_by: number | null;
	pos: { x: number; y: number; z: number } | null;
	respawn_in_ticks: number | null;
}): PowerItemStatus {
	return {
		spawn_id: p.spawn_id,
		status: p.status as PowerItemStatus['status'],
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
		// v2 dropped these; default to 0 so the table column still renders.
		unk_damage_1: 0,
		time_existing: o.time_existing,
		// v1 used 0xFFFFFFFF as the no-owner sentinel; v2 collapses to null.
		// Reverse for v1 consumers that pattern-match on the sentinel.
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
		// v2 dropped these fields — they didn't carry meaningful state from
		// the underlying memory walk; default to 0 so the v1 table renders.
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
					// v2 dropped these on the wire — the field still exists in the v1
					// type so a tile renders; surface 0 / sentinel for absent values.
					game_difficulty_level: 0,
					stored_global_random: tick.game_globals.stored_global_random
				}
			: null,
		player_count: tick?.players?.length ?? 0,
		local_count: tick?.locals?.length ?? 0,
		locals:
			tick?.locals?.map((l) => ({
				local_index: l.local_index,
				// v2's TickFPWeapon is slimmer; what's present projects directly.
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
				// v2 trimmed input — most fields are gone. Leave as null; the
				// v1 sub-section renders an empty card with the field labels.
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
		// v1 had a data_queue tick struct; v2 doesn't expose it. The section
		// component renders "null" subtitle gracefully.
		data_queue: null,
		ctf_flags: tick?.ctf_flags?.map(v2CTFToV1) ?? null,
		objects: objects?.objects?.map(v2ObjectToV1) ?? null,
		projectiles: objects?.projectiles?.map(v2ProjectileToV1) ?? null
	};
}

export function buildTickVm(name: string, ws: ScraperWSV2, ctx: DebugContext): TickVm {
	const v2Tick = ws.tick[name] ?? null;
	const v2Objects = ws.objects[name] ?? null;
	const v2Game = ws.game[name] ?? null;
	const projected = v2ToV1Tick(v2Tick, v2Objects, v2Game);
	const tick = projected ?? ctx.inspect?.latest_tick ?? null;
	const phase: Phase = ((v2Game?.phase ?? ctx.inspect?.phase) as Phase | undefined) ?? 'idle';
	const tickValue = ws.engineTick[name] ?? ctx.inspect?.tick;

	// Index roster players by tick-player index so a TickPlayer row can pull
	// its identity (name/team) without iterating the full roster each time.
	const playersByIndex = new Map<number, GamePlayer>();
	const roster = v2Game?.players ?? ctx.inspect?.game_data?.players ?? [];
	for (const p of roster) playersByIndex.set(p.index, p);

	return {
		tick,
		phase,
		tickValue,
		tickStr: ctx.relativeTime(ws.tickAt[name]),
		isTeamGame: v2Game?.config?.is_team_game ?? ctx.inspect?.game_data?.is_team_game === true,
		playersByIndex
	};
}
