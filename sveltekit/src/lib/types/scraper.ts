// Types mirroring internal/scraper/types.go JSON tags. Hand-written because the
// scraper does not back a PocketBase collection — task typegen does not cover
// these.

export type GameState = 'menu' | 'pregame' | 'in_game' | 'postgame';

export type EnvelopeType = 'snapshot' | 'tick' | 'event';

// Outer WebSocket Message wrapper. The scraper broadcasts always set
// type="scraper", room="overlay", and payload=<Envelope as JSON>.
export interface WSMessage {
	type: string;
	room?: string;
	target?: string;
	payload?: unknown;
}

// Inner scraper envelope. Payload shape depends on type.
export interface Envelope<P = unknown> {
	type: EnvelopeType;
	instance: string;
	tick: number;
	payload: P;
}

export interface TeamScore {
	team: number;
	score: number;
}

export interface SnapshotPlayer {
	index: number;
	name: string;
	team: number;
	kills: number;
	deaths: number;
	assists: number;
	ctf_score: number;
	team_kills: number;
	suicides: number;
	kill_streak: number;
	multikill: number;
	shots_fired: number;
	shots_hit: number;
	is_local: boolean | null;
	local_index: number | null;
}

export interface PowerItemSpawn {
	spawn_id: number;
	tag: string;
	spawn_interval_ticks: number;
	x: number;
	y: number;
	z: number;
}

export interface SnapshotPayload {
	game_state: GameState;
	map: string;
	gametype: string;
	is_team_game: boolean;
	score_limit: number;
	time_limit_ticks: number;
	team_scores: TeamScore[] | null;
	players: SnapshotPlayer[] | null;
	power_item_spawns: PowerItemSpawn[] | null;
}

export interface WeaponInfo {
	slot: number;
	object_id: number;
	tag: string;
	ammo_pack: number | null;
	ammo_mag: number | null;
	charge?: number | null;
	is_energy: boolean;
}

export interface TickPlayer {
	index: number;
	alive: boolean;
	respawn_in_ticks: number | null;
	x: number;
	y: number;
	z: number;
	vx: number;
	vy: number;
	vz: number;
	aim_x: number;
	aim_y: number;
	aim_z: number;
	zoom_level: number;
	crouchscale: number;
	health: number;
	shields: number;
	has_camo: boolean;
	has_overshield: boolean;
	frags: number;
	plasmas: number;
	selected_weapon_slot: number;
	is_crouching: boolean;
	is_jumping: boolean;
	is_firing: boolean;
	is_shooting: boolean;
	is_flashlight_on: boolean;
	is_throwing_grenade: boolean;
	is_meleeing: boolean;
	is_pressing_action: boolean;
	is_holding_action: boolean;
	weapons: WeaponInfo[] | null;
}

export interface XYZ {
	x: number;
	y: number;
	z: number;
}

export interface PowerItemStatus {
	spawn_id: number;
	status: 'held' | 'world' | 'respawning';
	held_by: number | null;
	world_pos: XYZ | null;
	respawn_in_ticks: number | null;
}

export interface TickPayload {
	players: TickPlayer[] | null;
	power_items: PowerItemStatus[] | null;
}

// Type guards for narrowing Envelope by type.
export function isSnapshot(env: Envelope): env is Envelope<SnapshotPayload> {
	return env.type === 'snapshot';
}

export function isTick(env: Envelope): env is Envelope<TickPayload> {
	return env.type === 'tick';
}
