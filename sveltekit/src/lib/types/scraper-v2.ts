// V2 wire-protocol types — mirrors internal/scraper/types.go and the
// per-class payload structs in internal/scraper/manager/ (xbox.go,
// scenario.go, game.go, tick.go, objects.go, debug.go, summary.go,
// previous_game.go, hello.go) plus event payloads in
// internal/scraper/event_payloads.go.
//
// Separate file from scraper.ts (the v1 types) so PR 19 ships without
// touching the v1 surface. PR 20 swaps the WS store to v2; PR 21+ moves
// consumers across; PR 26 deletes scraper.ts and renames this file.
//
// Design reference: atlas/new_json/04-ground-up-rebuild.md.

// =============================================================================
// Envelope + wrapper
// =============================================================================

/** Protocol version carried on every Envelope.v. Backend bumps on
 * breaking payload changes; clients should branch or warn. */
export const PROTOCOL_VERSION_V2 = 2;

/** Every class + control envelope kind the server emits. State classes
 * (xbox..summary), per-event live emissions (event), addressed event
 * backfills (events), the connect handshake (hello), and the error
 * channel. */
export type EnvelopeTypeV2 =
	| 'xbox'
	| 'scenario'
	| 'game'
	// game_filtered: the viewer-facing game class, dummy-filtered server-side.
	// Same GamePayload shape as 'game'; overlays subscribe to this room and the
	// client stores it in the normal `game` slot (see scraper-ws-v2 handleEnvelope).
	| 'game_filtered'
	| 'tick'
	| 'objects'
	| 'debug'
	| 'probe'
	| 'summary'
	| 'previous_game'
	| 'event'
	| 'events'
	| 'hello'
	| 'error';

/** Inner v2 envelope — the JSON the server emits as the `payload` of
 * the outer WebSocket Message. seq is uint64 (string-safe via JS number
 * up to 2^53; tracked on backend per-(instance, type)). tick is the
 * engine tick on per-tick classes; 0 for non-tick-driven payloads. */
export interface EnvelopeV2<P = unknown> {
	v: number;
	type: EnvelopeTypeV2;
	instance: string;
	seq: number;
	tick: number;
	ts: string;
	data: P;
}

// =============================================================================
// Sub-shapes
// =============================================================================

/** Canonical 3D vector. Used for positions, velocities, aim directions. */
export interface Vec3 {
	x: number;
	y: number;
	z: number;
}

/** Canonical 2D vector. Used for stick coordinates and screen-space points. */
export interface Vec2 {
	x: number;
	y: number;
}

/** Identity-only player reference. Positions live as peer fields on the
 * carrying event (`pos`, `victim_pos`, `killer_pos`). Identity is
 * denormalised so an event log alone is sufficient for analytics — a
 * later team switch does not retroactively change what was true. */
export interface PlayerRef {
	index: number;
	name: string;
	team: number;
	armor_color: number;
}

/** Identity-only world-item reference. spawn_id is optional: dropped
 * weapons have no scenario spawn, while spawn_id=0 IS valid (first
 * spawn). Use `'spawn_id' in ref` to distinguish absent from zero. */
export interface ItemRef {
	spawn_id?: number;
	tag: string;
}

/** Identity-only vehicle reference. Seat (driver/passenger/gunner)
 * lives as a peer field on vehicle_entered, not on the ref. */
export interface VehicleRef {
	object_id: number;
	tag: string;
}

// =============================================================================
// Hello (handshake)
// =============================================================================

export interface HelloPayloadV2 {
	protocol_version: number;
	server_time: string;
	classes: string[];
	instances: HelloInstanceV2[];
}

export interface HelloInstanceV2 {
	name: string;
	/** ISO timestamp; advances on runner restart so clients can detect
	 * stale per-(instance, type) seq caches and resubscribe. */
	started_at: string;
}

// =============================================================================
// xbox class — machine + XBE identity + kernel clock
// =============================================================================

export interface XboxPayload {
	title_id: number;
	title: string;
	name: string;
	serial_number: string;
	mac_address: string;
	video_standard: string;
	time_zone: XboxTimeZone | null;
	xbe: XboxXBE | null;
	kernel: XboxKernel | null;
}

/** Nested so bias_minutes=0 (GMT) survives on the wire — the outer
 * object's presence signals "read", not any one field's non-zero value. */
export interface XboxTimeZone {
	bias_minutes: number;
	std_name: string;
	dlt_name: string;
}

/** Nested so zero-valid fields (disk_number=0) persist. */
export interface XboxXBE {
	title_name: string;
	version: number;
	game_region: number;
	disk_number: number;
	allowed_media: number;
}

/** uptime_seconds is a float for readability vs v1's 14-digit ns int. */
export interface XboxKernel {
	system_time: string;
	boot_time: string;
	uptime_seconds: number;
}

// =============================================================================
// scenario class — loaded map; static, one envelope per map load
// =============================================================================

export interface ScenarioPayload {
	map: string;
	game_difficulty: number;
	fog: ScenarioFog | null;
	memory_regions: ScenarioMemoryRegions | null;
	object_types: ScenarioObjectType[];
	player_spawns: ScenarioPlayerSpawn[];
	power_item_spawns: ScenarioPowerItemSpawn[];
	/** Keyed by tag string ("weapons\\pistol\\pistol", "characters\\cyborg\\cyborg").
	 * Tick refers to tags by string; static data lives here. */
	tag_defs: Record<string, ScenarioTagDef>;
}

export interface ScenarioFog {
	color: { r: number; g: number; b: number };
	max_density: number;
	atmo_min_dist: number;
	atmo_max_dist: number;
}

/** Replaces the misleadingly-named v1 `tag_cache` field — these are
 * memory base/size pointers, not a cache of tags. */
export interface ScenarioMemoryRegions {
	game_state: ScenarioMemoryRegion;
	tag_cache: ScenarioMemoryRegion;
	texture_cache: ScenarioMemoryRegion;
	sound_cache: ScenarioMemoryRegion;
}

export interface ScenarioMemoryRegion {
	base: number;
	size: number;
}

export interface ScenarioObjectType {
	type_index: number;
	name: string;
	datum_size: number;
}

export interface ScenarioPlayerSpawn {
	index: number;
	pos: Vec3;
	facing: number;
	team_index: number;
	bsp_index: number;
	gametypes: number[];
}

export interface ScenarioPowerItemSpawn {
	spawn_id: number;
	tag: string;
	interval_ticks: number;
	/** u8 bitmask of which gametypes this placement applies to; 0 = no
	 * gametype constraint. The same physical position may appear in
	 * multiple rows with different `gametype_mask` + `interval_ticks`
	 * because Halo CE stores per-gametype variants of placements (e.g.
	 * CTF rocket at 30s respawn vs Slayer rocket at 120s respawn, same
	 * coords). Bit-to-gametype mapping is not yet confirmed — see the
	 * 2026-05-19 planning entry in docs/milestones/M19-robustness-offsets.md. */
	gametype_mask: number;
	pos: Vec3;
}

/** Discriminated union keyed on `kind`. Today: weapon | biped. Other
 * `kind` values may add fields with optional types; consumers should
 * narrow with `if (def.kind === 'weapon')` before reading specific
 * fields. */
export interface ScenarioTagDef {
	kind: 'weapon' | 'biped' | string;

	// weapon fields
	zoom_levels?: number;
	zoom_min?: number;
	zoom_max?: number;
	autoaim_angle?: number;
	autoaim_range?: number;
	magnetism_angle?: number;
	magnetism_range?: number;
	deviation_angle?: number;

	// biped fields
	tag_index?: number;
	flags?: number;
	autoaim_pill_radius?: number;
}

// =============================================================================
// game class — phase, config, roster, scores, machines, network
// =============================================================================

export type PhaseV2 = 'idle' | 'ready' | 'live';

export interface GamePayload {
	phase: PhaseV2;
	started_at: string;
	last_read_at: string;
	/** The real match clock (GTG 0x0C): 30Hz count-up, re-inits to 0 at match
	 * start — the scorebug renders this as M:SS while phase==='live'. (At the
	 * menu it free-runs, hence the phase gate.) LIVE-VERIFIED it ticks 30/s. */
	engine_tick: number;
	/** GTG 0x10 (offset-mapper's inferred match-elapsed) — LIVE-VERIFIED STUCK at
	 * ~1, does NOT tick during play, so NOT used for the clock. Kept for now. */
	game_elapsed_ticks?: number;
	iterations: number;

	config: GameConfig | null;
	team_scores: GameTeamScore[];
	players: GameRosterPlayer[];
	machines: GameMachine[];
	network: GameNetwork | null;
}

export interface GameConfig {
	gametype: string;
	variant_name?: string;
	is_team_game: boolean;
	score_limit: number;
	time_limit_ticks: number;
}

export interface GameTeamScore {
	team: number;
	score: number;
}

export interface GameRosterPlayer {
	index: number;
	name: string;
	team: number;
	armor_color: number;
	score: number;
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
	machine_index: number | null;
	controller_index: number | null;
	/** Accumulated match stats (HaloCaster extract_events port, server-side).
	 * The engine's shots_fired/shots_hit above read 0 live — acc_* are the
	 * working tick-delta equivalents; best_kill_streak is the match PEAK.
	 * Optional: absent from pre-accumulator servers; consumers default to 0. */
	acc_shots_fired?: number;
	acc_grenade_throws?: number;
	acc_melees?: number;
	acc_damage_dealt?: number;
	acc_damage_received?: number;
	acc_camo_pickups?: number;
	acc_overshield_pickups?: number;
	best_kill_streak?: number;
}

export interface GameMachine {
	index: number;
	name: string;
	is_local: boolean | null;
}

/** Moved from v1's TickPayload — slow-changing, no need for 30 Hz cadence. */
export interface GameNetwork {
	countdown: GameNetworkCountdown | null;
	client: GameNetworkClient | null;
}

export interface GameNetworkCountdown {
	active: boolean;
	paused: boolean;
	seconds_to_start: number;
}

export interface GameNetworkClient {
	machine_index: number;
	average_ping: number;
	packets_sent: number;
}

// =============================================================================
// tick class — per-tick hot path (~30 Hz)
// =============================================================================

export interface TickPayloadV2 {
	players: TickPlayerV2[];
	power_items: TickPowerItem[];
	ctf_flags: TickCTFFlag[];
	game_globals: TickGameGlobals | null;
	locals: TickLocal[];
}

/** Slimmer than v1's TickPlayer — the 66-field `extended` block, `bones`,
 * and `update_queue` moved to the debug class; per-weapon static `tag_data`
 * and inline `biped_tag` moved to scenario.tag_defs. */
export interface TickPlayerV2 {
	index: number;
	alive: boolean;
	respawn_in_ticks: number | null;
	pos: Vec3;
	vel: Vec3;
	aim: Vec3;
	zoom_level: number;
	crouch_scale: number;
	/** 0..1 engine body_vitality fraction (1.0 = full). */
	health: number;
	/** 0..1 (can read >1 with overshield). */
	shields: number;
	/** Absolute health/shield ceilings in engine units (~75). */
	max_health: number;
	max_shields: number;
	has_camo: boolean;
	has_overshield: boolean;
	frags: number;
	plasmas: number;
	selected_weapon_slot: number;
	biped_tag: string;
	actions: TickPlayerActions;
	weapons: TickWeaponV2[];
}

/** Collapses v1's 9-boolean soup. is_firing+is_shooting → firing;
 * is_pressing_action+is_holding_action → using. Fine-grained distinctions
 * live in the debug class. */
export interface TickPlayerActions {
	crouching: boolean;
	jumping: boolean;
	firing: boolean;
	flashlight: boolean;
	throwing_grenade: boolean;
	meleeing: boolean;
	using: boolean;
}

/** Dynamic per-weapon state only — static tag_data lives in
 * scenario.tag_defs keyed by `tag`. */
export interface TickWeaponV2 {
	slot: number;
	tag: string;
	object_id: number;
	ammo_mag: number | null;
	ammo_pack: number | null;
	charge: number | null;
	heat: number;
	reloading: boolean;
}

export interface TickPowerItem {
	spawn_id: number;
	status: 'held' | 'world' | 'respawning';
	held_by: number | null;
	pos: Vec3 | null;
	respawn_in_ticks: number | null;
}

export interface TickCTFFlag {
	team: number;
	pos: Vec3;
	carrier: number | null;
	status: 'home' | 'carried' | 'dropped' | string;
}

export interface TickGameGlobals {
	map_loaded: number;
	active: number;
	game_loading_in_progress: number;
	precache_map_status: number;
	stored_global_random: number;
}

export interface TickLocal {
	local_index: number;
	fp_weapon?: TickFPWeapon | null;
	observer_cam?: TickObserverCam | null;
	input?: TickLocalInput | null;
	player_control?: TickPlayerControl | null;
}

export interface TickFPWeapon {
	state: number;
	animation_id: number;
	animation_tick: number;
}

export interface TickObserverCam {
	pos: Vec3;
	aim: Vec3;
	fov: number;
}

/** Abstracted input — stick coords + the buttons load-bearing UIs read.
 * Full gamepad state (per-button durations, full d-pad) lives in the
 * debug class. */
export interface TickLocalInput {
	left_stick: Vec2;
	right_stick: Vec2;
	buttons: TickLocalInputButtons;
}

export interface TickLocalInputButtons {
	a: boolean;
	b: boolean;
}

export interface TickPlayerControl {
	desired_yaw: number;
	desired_pitch: number;
	zoom_level: number;
	aim_assist_target: number | null;
}

// =============================================================================
// objects class — world objects + projectiles (opt-in, per-tick)
// =============================================================================

export interface ObjectsPayload {
	objects: WorldObject[];
	projectiles: Projectile[];
}

export interface WorldObject {
	object_id: number;
	tag: string;
	type: number;
	flags: number;
	pos: Vec3;
	ang_vel: Vec3;
	time_existing: number;
	/** null when no owning unit (the 0xFFFFFFFF sentinel collapses to null). */
	owner_unit: number | null;
	/** null when no owning object. */
	owner_object: number | null;
	ultimate_parent: number;
}

export interface Projectile {
	object_id: number;
	tag: string;
	pos: Vec3;
	flags: number;
	action: number;
	detonation_timer: number;
	distance_traveled: number;
}

// =============================================================================
// debug class — reverse-engineered fields, opt-in
// =============================================================================

export interface DebugPayload {
	players: DebugPlayer[];
}

// =============================================================================
// probe class — on-demand probe values, request/reply only
// =============================================================================

/** Probe is the developer scratch space for finding/debugging memory
 * values. Never broadcast — emitted only in reply to a request_probe
 * WS message. Both fields are populated by the active GameReader
 * plugin's LastStateInputs / BuildScoreProbe methods on the runner
 * loop goroutine. */
export interface ProbePayload {
	/** Free-form plugin-defined state inputs. Halo: CE keys include
	 * main_menu, initialized, active, paused, engine_running, etc. */
	state_inputs: Record<string, unknown>;
	/** Plugin-defined gametype / score probes — six top-level keys for
	 * Halo: CE (gametype_candidates, team_scores_raw, score_limits_raw,
	 * per_player_score_tables, per_player_static_struct,
	 * per_player_biped_regions). */
	score_probe: Record<string, unknown>;
}

export interface DebugPlayer {
	index: number;
	extended?: DebugPlayerExtended | null;
	bones?: DebugBone[] | null;
	update_queue?: DebugUpdateQueue | null;
	/** Undecoded biped-extended fields keyed by hex memory offset
	 * (e.g. "0x98"). Fields migrate to DebugPlayerExtended as their
	 * semantics are figured out. */
	raw?: Record<string, unknown> | null;
}

export interface DebugPlayerExtended {
	legs: DebugLegRotation;
	airborne: boolean;
	airborne_ticks: number;
	idle_ticks: number;
	state_flags: number;
	stunned: number;
	aim_assist_sphere?: DebugSphere | null;
}

export interface DebugLegRotation {
	pitch: number;
	yaw: number;
	roll: number;
}

export interface DebugSphere {
	center: Vec3;
	radius: number;
}

export interface DebugBone {
	index: number;
	pos: Vec3;
}

export interface DebugUpdateQueue {
	buttons: DebugUpdateQueueButtons;
	desired_yaw: number;
	desired_pitch: number;
}

export interface DebugUpdateQueueButtons {
	crouch: boolean;
	jump: boolean;
	fire: boolean;
}

// =============================================================================
// summary class — cross-instance dashboard feed
// =============================================================================

export interface SummaryPayload {
	hosts: HostSummaryV2[];
}

export interface HostSummaryV2 {
	instance: string;
	phase: PhaseV2;
	title: string;
	xbe_title_name: string;
	map: string;
	gametype: string;
	score_summary: string;
	last_successful_read_at: string;
}

// =============================================================================
// previous_game class — snapshot + event log of the just-ended match
// =============================================================================

export interface PreviousGamePayload {
	ended_at: string;
	game: GamePayload | null;
	events: EnvelopeV2[];
}

// =============================================================================
// Event payloads (envelope.type === 'event' or 'events')
// =============================================================================

/** event_type constants — top-level discriminator in every event payload. */
export const EventType = {
	Death: 'death',
	Damage: 'damage',
	Medal: 'medal',
	PlayerUpdate: 'player_update',
	GameUpdate: 'game_update'
} as const;
export type EventType = (typeof EventType)[keyof typeof EventType];

export const DeathCause = {
	Kill: 'kill',
	Suicide: 'suicide',
	Fall: 'fall',
	Betrayal: 'betrayal',
	Environment: 'environment',
	Unknown: 'unknown'
} as const;
export type DeathCause = (typeof DeathCause)[keyof typeof DeathCause];

export const DamageKind = {
	Hit: 'hit',
	Melee: 'melee'
} as const;
export type DamageKind = (typeof DamageKind)[keyof typeof DamageKind];

export const MedalKind = {
	Multikill: 'multikill',
	KillStreak: 'kill_streak'
} as const;
export type MedalKind = (typeof MedalKind)[keyof typeof MedalKind];

export const PlayerUpdateKind = {
	Spawn: 'spawn',
	Score: 'score',
	PowerupPickedUp: 'powerup_picked_up',
	PowerupExpired: 'powerup_expired',
	VehicleEntered: 'vehicle_entered',
	VehicleExited: 'vehicle_exited',
	ItemPickedUp: 'item_picked_up',
	ItemDropped: 'item_dropped',
	ItemDepleted: 'item_depleted',
	GrenadeThrown: 'grenade_thrown',
	PlayerQuit: 'player_quit'
} as const;
export type PlayerUpdateKind = (typeof PlayerUpdateKind)[keyof typeof PlayerUpdateKind];

export const GameUpdateKind = {
	PlayerJoined: 'player_joined',
	PlayerLeft: 'player_left',
	PlayerTeamChanged: 'player_team_changed',
	TeamScore: 'team_score',
	GameStart: 'game_start',
	GameEnd: 'game_end',
	ItemSpawned: 'item_spawned'
} as const;
export type GameUpdateKind = (typeof GameUpdateKind)[keyof typeof GameUpdateKind];

/** EventCommon is embedded into every event payload via field merge
 * (Go struct embedding flattens to top-level JSON). On the wire each
 * event payload carries all four fields plus its kind-specific extras. */
export interface EventCommon {
	seq: number;
	tick: number;
	at: string;
	event_type: EventType;
}

export interface DeathEvent extends EventCommon {
	event_type: 'death';
	victim: PlayerRef;
	victim_pos: Vec3;
	killer: PlayerRef | null;
	killer_pos: Vec3 | null;
	cause: DeathCause;
	weapon: string;
	team_kill: boolean;
	respawn_in_ticks: number;
}

export interface DamageEvent extends EventCommon {
	event_type: 'damage';
	kind: DamageKind;
	dealer: PlayerRef | null;
	dealer_pos: Vec3 | null;
	victim: PlayerRef;
	victim_pos: Vec3;
	amount: number;
	weapon: string;
}

export interface MedalEvent extends EventCommon {
	event_type: 'medal';
	kind: MedalKind;
	player: PlayerRef;
	count: number;
}

export interface PlayerUpdateEvent extends EventCommon {
	event_type: 'player_update';
	kind: PlayerUpdateKind;
	player: PlayerRef;
	pos?: Vec3;
	// kind=score
	kills?: number;
	deaths?: number;
	assists?: number;
	kill_streak?: number;
	multikill?: number;
	// kind=powerup_*
	powerup?: string;
	// kind=vehicle_*
	vehicle?: VehicleRef;
	seat?: string;
	// kind=item_*
	item?: ItemRef;
	// kind=grenade_thrown
	grenade_type?: string;
	remaining?: number;
	// kind=item_depleted
	cause?: 'ammo' | 'energy';
}

export interface GameUpdateEvent extends EventCommon {
	event_type: 'game_update';
	kind: GameUpdateKind;
	player?: PlayerRef;
	previous_team?: number;
	team?: number;
	score?: number;
	map?: string;
	gametype?: string;
	item?: ItemRef;
	pos?: Vec3;
}

/** Discriminated union over every event payload — narrow with
 * `if (ev.event_type === 'death')`. */
export type AnyEvent = DeathEvent | DamageEvent | MedalEvent | PlayerUpdateEvent | GameUpdateEvent;

/** events backfill reply (envelope.type === 'events') — addressed to
 * the requester only. since_tick is echoed so the requester can
 * reconcile against an outstanding request. */
export interface EventsReplyPayload {
	phase: PhaseV2;
	since_tick: number;
	/** Oldest-first per backend OQ7. Each entry is a full Envelope wrapping
	 * one of the event payloads above. */
	events: EnvelopeV2[];
}

// =============================================================================
// Room helpers
// =============================================================================

/** host:<instance>:<class> — per-instance per-class subscription room. */
export function roomForInstanceClass(instance: string, cls: EnvelopeTypeV2): string {
	return `host:${instance}:${cls}`;
}

/** Cross-instance summary room. */
export const SUMMARY_ROOM = 'host:summary';

// =============================================================================
// Type guards
// =============================================================================

export function isXboxEnv(e: EnvelopeV2): e is EnvelopeV2<XboxPayload> & { type: 'xbox' } {
	return e.type === 'xbox';
}
export function isScenarioEnv(
	e: EnvelopeV2
): e is EnvelopeV2<ScenarioPayload> & { type: 'scenario' } {
	return e.type === 'scenario';
}
export function isGameEnv(e: EnvelopeV2): e is EnvelopeV2<GamePayload> & { type: 'game' } {
	return e.type === 'game';
}
/** game_filtered carries a GamePayload just like game (dummy-filtered server-side). */
export function isGameFilteredEnv(
	e: EnvelopeV2
): e is EnvelopeV2<GamePayload> & { type: 'game_filtered' } {
	return e.type === 'game_filtered';
}
export function isTickEnv(e: EnvelopeV2): e is EnvelopeV2<TickPayloadV2> & { type: 'tick' } {
	return e.type === 'tick';
}
export function isObjectsEnv(e: EnvelopeV2): e is EnvelopeV2<ObjectsPayload> & { type: 'objects' } {
	return e.type === 'objects';
}
export function isProbeEnv(e: EnvelopeV2): e is EnvelopeV2<ProbePayload> & { type: 'probe' } {
	return e.type === 'probe';
}
export function isDebugEnv(e: EnvelopeV2): e is EnvelopeV2<DebugPayload> & { type: 'debug' } {
	return e.type === 'debug';
}
export function isSummaryEnv(e: EnvelopeV2): e is EnvelopeV2<SummaryPayload> & { type: 'summary' } {
	return e.type === 'summary';
}
export function isPreviousGameEnv(
	e: EnvelopeV2
): e is EnvelopeV2<PreviousGamePayload> & { type: 'previous_game' } {
	return e.type === 'previous_game';
}
export function isEventEnv(e: EnvelopeV2): e is EnvelopeV2<AnyEvent> & { type: 'event' } {
	return e.type === 'event';
}
export function isEventsReplyEnv(
	e: EnvelopeV2
): e is EnvelopeV2<EventsReplyPayload> & { type: 'events' } {
	return e.type === 'events';
}
export function isHelloEnv(e: EnvelopeV2): e is EnvelopeV2<HelloPayloadV2> & { type: 'hello' } {
	return e.type === 'hello';
}
