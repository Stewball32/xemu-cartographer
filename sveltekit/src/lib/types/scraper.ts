// Types mirroring internal/scraper/types.go JSON tags. Hand-written because the
// scraper does not back a PocketBase collection — task typegen does not cover
// these.

// Mirrors internal/guards/interfaces/scraper.Info — one row per running runner
// returned by GET /api/admin/scraper. Includes orphan instances (sockets that
// auto-attached via the discovery watcher without a backing container record).
export interface ScraperInfo {
	name: string;
	sock: string;
	title_id: number;
	game_title: string;
	xbox_name: string;
	tick: number;
	ticks: number;
	started_at: string;
}

// Mirrors internal/guards/interfaces/scraper.InspectState — returned by
// GET /api/admin/scraper/{name}/inspect. Embeds ScraperInfo plus the runner's
// cached snapshot/tick/state/events for the debug page. Values are nullable
// when the runner has been alive but never observed an in-game tick or
// snapshot-eligible state transition.
export interface ScraperInspect extends ScraperInfo {
	running: boolean;
	current_state: GameState | '';
	state_inputs: StateInputs | null;
	score_probe: ScoreProbe | null;
	latest_snapshot: SnapshotPayload | null;
	latest_tick: TickPayload | null;
	recent_events: Envelope[] | null;
}

// StateInputs is a free-form bag of plugin-specific raw values that drive
// state classification — keys depend on which game plugin is active. For
// Halo: CE: main_menu, initialized, active, paused, engine_running,
// game_can_score, game_engine_globals_ptr, game_time_globals_ptr.
export type StateInputs = Record<string, number | boolean | string | null>;

// ScoreProbe is a free-form bag of every candidate address the active plugin
// reads for gametype / team-score / per-player-score detection. Surfaced via
// the inspect endpoint and rendered as the debug page's Probe tab so a human
// can spot which raw value matches what they see in-game.
export type ScoreProbe = Record<string, unknown>;

export type GameState = 'menu' | 'lobby' | 'pregame' | 'in_game' | 'postgame';

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
}

export interface SnapshotMachine {
	index: number;
	name: string;
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
	variant_name?: string;
	is_team_game: boolean;
	score_limit: number;
	time_limit_ticks: number;
	team_scores: TeamScore[] | null;
	players: SnapshotPlayer[] | null;
	power_item_spawns: PowerItemSpawn[] | null;
	machines?: SnapshotMachine[] | null;

	// Static map / scenario data scraped at match-start.
	game_difficulty: number;
	player_spawns?: StaticPlayerSpawn[] | null;
	fog?: StaticFog | null;
	object_types?: StaticObjectType[] | null;
	tag_cache?: StaticCachePtrs | null;
}

export interface StaticPlayerSpawn {
	index: number;
	x: number;
	y: number;
	z: number;
	facing: number;
	team_index: number;
	bsp_index: number;
	unk_0: number;
	gametype_0: number;
	gametype_1: number;
	gametype_2: number;
	gametype_3: number;
}

export interface StaticFog {
	color_r: number;
	color_g: number;
	color_b: number;
	max_density: number;
	atmo_min_dist: number;
	atmo_max_dist: number;
}

export interface StaticObjectType {
	type_index: number;
	name: string;
	datum_size: number;
}

export interface StaticCachePtrs {
	game_state_base: number;
	game_state_size: number;
	tag_cache_base: number;
	tag_cache_size: number;
	texture_cache_base: number;
	texture_cache_size: number;
	sound_cache_base: number;
	sound_cache_size: number;
}

export interface WeaponInfo {
	slot: number;
	object_id: number;
	tag: string;
	ammo_pack: number | null;
	ammo_mag: number | null;
	charge?: number | null;
	is_energy: boolean;

	extended?: TickWeaponObjectExtended | null;
	tag_data?: StaticWeaponTagData | null;
}

export interface TickWeaponObjectExtended {
	heat_meter: number;
	used_energy: number;
	owner_handle: number;
	is_reloading: number;
	reload_time: number;
	can_fire: number;
	world?: XYZ | null;
}

export interface StaticWeaponTagData {
	zoom_levels: number;
	zoom_min: number;
	zoom_max: number;
	autoaim_angle: number;
	autoaim_range: number;
	magnetism_angle: number;
	magnetism_range: number;
	deviation_angle: number;
	animations?: AnimEntry[] | null;
}

export interface AnimEntry {
	index: number;
	length: number;
	unk_46: number;
	unk_52: number;
	unk_54: number;
}

export interface StaticBipedTagData {
	tag_index: number;
	flags: number;
	autoaim_pill_radius: number;
}

// TickPlayer carries only high-frequency volatile data per game tick.
// Roster identity (name, team, splitscreen index) and cumulative counters
// (kills, deaths, assists, etc.) live on SnapshotPlayer. Counter changes
// are emitted via per-event payloads (kill / death / score / etc.) rather
// than streamed every tick.
export interface TickPlayer {
	index: number;

	// Dynamic per-tick state.
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

	extended?: TickPlayerExtended | null;
	bones?: TickBone[] | null;
	update_queue?: TickUpdateQueue | null;
	biped_tag?: StaticBipedTagData | null;
}

export interface TickBone {
	index: number;
	x: number;
	y: number;
	z: number;
}

export interface TickPlayerExtended {
	legs_pitch: number;
	legs_yaw: number;
	legs_roll: number;
	pitch_1: number;
	yaw_1: number;
	roll_1: number;
	ang_vel_x: number;
	ang_vel_y: number;
	ang_vel_z: number;
	aim_assist_sphere_x: number;
	aim_assist_sphere_y: number;
	aim_assist_sphere_z: number;
	aim_assist_sphere_radius: number;
	scale: number;
	type_u16: number;
	render_flags: number;
	weapon_owner_team: number;
	powerup_unk_2: number;
	idle_ticks: number;
	animation_unk_1: number;
	animation_unk_2: number;
	animation_unk_3: number;
	dmg_countdown_98: number;
	dmg_countdown_9c: number;
	dmg_countdown_a4: number;
	dmg_countdown_a8: number;
	dmg_counter_ac: number;
	dmg_counter_b0: number;
	shields_status_2: number;
	shields_charge_delay: number;
	next_object: number;
	next_object_2: number;
	state_flags: number;
	flashlight: number;
	stunned: number;
	x_unk_0: number;
	y_unk_0: number;
	z_unk_0: number;
	x_aim_a: number;
	y_aim_a: number;
	z_aim_a: number;
	x_aim_0: number;
	y_aim_0: number;
	z_aim_0: number;
	x_aim_1: number;
	y_aim_1: number;
	z_aim_1: number;
	looking_vector_x: number;
	looking_vector_y: number;
	looking_vector_z: number;
	move_forward: number;
	move_left: number;
	move_up: number;
	melee_damage_type: number;
	animation_1: number;
	animation_2: number;
	current_equipment: number;
	camo_self_revealed: number;
	facing_1: number;
	facing_2: number;
	facing_3: number;
	airborne: number;
	landing_stun_current_duration: number;
	landing_stun_target_duration: number;
	airborne_ticks: number;
	slipping_ticks: number;
	stop_ticks: number;
	jump_recovery_timer: number;
	landing: number;
	air_state_460: number;
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

	game_globals?: TickGameGlobals | null;
	player_count: number;
	local_count: number;
	locals?: TickLocal[] | null;
	network?: TickNetwork | null;
	data_queue?: TickDataQueue | null;
	ctf_flags?: TickCTFFlag[] | null;
	objects?: TickObject[] | null;
	projectiles?: TickProjectile[] | null;
}

export interface TickGameGlobals {
	map_loaded: number;
	active: number;
	players_are_double_speed: number;
	game_loading_in_progress: number;
	precache_map_status: number;
	game_difficulty_level: number;
	stored_global_random: number;
}

export interface TickLocal {
	local_index: number;
	fp_weapon?: TickFPWeapon | null;
	observer_cam?: TickObserverCam | null;
	ias?: TickInputAbstract | null;
	gamepad?: TickGamepad | null;
	ui?: TickUIGlobals | null;
	player_control?: TickPlayerControl | null;
	look_yaw_rate: number;
	look_pitch_rate: number;
}

export interface TickFPWeapon {
	weapon_rendered: number;
	player_object: number;
	weapon_object: number;
	state: number;
	idle_animation_threshold: number;
	idle_animation_counter: number;
	animation_id: number;
	animation_tick: number;
}

export interface TickObserverCam {
	x: number;
	y: number;
	z: number;
	vel_x: number;
	vel_y: number;
	vel_z: number;
	aim_x: number;
	aim_y: number;
	aim_z: number;
	fov: number;
}

export interface TickInputAbstract {
	btn_a: number;
	btn_black: number;
	btn_x: number;
	btn_y: number;
	btn_b: number;
	btn_white: number;
	left_trigger: number;
	right_trigger: number;
	btn_start: number;
	btn_back: number;
	left_stick_button: number;
	right_stick_button: number;
	left_stick_vertical: number;
	left_stick_horizontal: number;
	right_stick_horizontal: number;
	right_stick_vertical: number;
}

export interface TickGamepad {
	btn_a: number;
	btn_b: number;
	btn_x: number;
	btn_y: number;
	btn_black: number;
	btn_white: number;
	left_trigger: number;
	right_trigger: number;
	btn_a_duration: number;
	btn_b_duration: number;
	btn_x_duration: number;
	btn_y_duration: number;
	black_duration: number;
	white_duration: number;
	lt_duration: number;
	rt_duration: number;
	dpad_up_duration: number;
	dpad_down_duration: number;
	dpad_left_duration: number;
	dpad_right_duration: number;
	left_stick_duration: number;
	right_stick_duration: number;
	left_stick_horizontal: number;
	left_stick_vertical: number;
	right_stick_horizontal: number;
	right_stick_vertical: number;
}

export interface TickUIGlobals {
	color: number;
	button_config: number;
	joystick_config: number;
	sensitivity: number;
	joystick_inverted: number;
	rumble_enabled: number;
	flight_inverted: number;
	autocenter_enabled: number;
	active_player_profile_index: number;
	joined_multiplayer_game: number;
}

export interface TickPlayerControl {
	desired_yaw: number;
	desired_pitch: number;
	zoom_level: number;
	aim_assist_target: number;
	aim_assist_near: number;
	aim_assist_far: number;
}

export interface TickUpdateQueue {
	unit_ref: number;
	button_field: number;
	action_field: number;
	buttons: UpdateQueueButtons;
	actions: UpdateQueueActions;
	desired_yaw: number;
	desired_pitch: number;
	forward: number;
	left: number;
	right_trigger_held: number;
	desired_weapon: number;
	desired_grenades: number;
	zoom_level: number;
}

export interface UpdateQueueButtons {
	crouch: boolean;
	jump: boolean;
	fire: boolean;
	flashlight: boolean;
	reload: boolean;
	melee: boolean;
}

export interface UpdateQueueActions {
	throw_grenade: boolean;
	use_action: boolean;
}

export interface TickNetwork {
	client?: TickNetworkClient | null;
	server?: TickNetworkServer | null;
	game_data?: TickNetworkGameData | null;
	machines?: TickNetMachine[] | null;
	network_players?: TickNetPlayer[] | null;
}

export interface TickNetworkClient {
	machine_index: number;
	ping_target_ip: number;
	packets_sent: number;
	packets_received: number;
	average_ping: number;
	ping_active: number;
	seconds_to_game_start: number;
}

export interface TickNetworkServer {
	countdown_active: number;
	countdown_paused: number;
	countdown_adjusted_time: number;
}

export interface TickNetworkGameData {
	maximum_player_count: number;
	machine_count: number;
	player_count: number;
}

export interface TickNetMachine {
	index: number;
	name: string;
}

export interface TickNetPlayer {
	name: string;
	color: number;
	unused: number;
	machine_index: number;
	controller_index: number;
	team: number;
	list_index: number;
}

export interface TickDataQueue {
	tick: number;
	global_random: number;
	tick2: number;
	unk1: number;
	player_count: number;
}

export interface TickCTFFlag {
	team: number;
	x: number;
	y: number;
	z: number;
	carrier_index: number | null;
	status: string;
}

export interface TickObject {
	object_id: number;
	tag: string;
	type: number;
	flags: number;
	x: number;
	y: number;
	z: number;
	ang_vel_x: number;
	ang_vel_y: number;
	ang_vel_z: number;
	unk_damage_1: number;
	time_existing: number;
	owner_unit_ref: number;
	owner_object_ref: number;
	ultimate_parent: number;
}

export interface TickProjectile {
	object_id: number;
	tag: string;
	x: number;
	y: number;
	z: number;
	flags: number;
	action: number;
	hit_material_type: number;
	ignore_object_index: number;
	detonation_timer: number;
	detonation_timer_delta: number;
	target_object_index: number;
	arming_time_delta: number;
	distance_traveled: number;
	deceleration_timer: number;
	deceleration_timer_delta: number;
	deceleration: number;
	maximum_damage_distance: number;
	rotation_axis_x: number;
	rotation_axis_y: number;
	rotation_axis_z: number;
	rotation_sine: number;
	rotation_cosine: number;
}

// Type guards for narrowing Envelope by type.
export function isSnapshot(env: Envelope): env is Envelope<SnapshotPayload> {
	return env.type === 'snapshot';
}

export function isTick(env: Envelope): env is Envelope<TickPayload> {
	return env.type === 'tick';
}
