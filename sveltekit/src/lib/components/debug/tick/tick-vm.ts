// Pretty-view VM for the Tick debug tab — formats the TickPayloadV2 into
// section records that mirror the envelope's JSON shape (players /
// power_items / ctf_flags / game_globals / locals). Every section is
// always present; payloads missing a field render as '—' so the Pretty
// view exposes the expected envelope structure even before the first read.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type {
	TickCTFFlag,
	TickGameGlobals,
	TickLocal,
	TickPayloadV2,
	TickPlayerActions,
	TickPlayerV2,
	TickPowerItem,
	TickWeaponV2,
	Vec3
} from '$lib/types/scraper-v2';

// One tile's display state + the raw value (passed through StatTile's
// `title` prop so hovering a formatted tile reveals the underlying
// wire value).
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

// Pretty-printed row for the players DataTable. Nested objects (pos /
// vel / aim / actions / weapons) are flattened to per-column scalars so
// the table doesn't have to call back into the VM for sub-formatting.
export interface PrettyPlayerRow {
	index: number;
	alive: boolean;
	respawn_in_ticks: string;
	pos_x: string;
	pos_y: string;
	pos_z: string;
	vel_x: string;
	vel_y: string;
	vel_z: string;
	aim_x: string;
	aim_y: string;
	aim_z: string;
	zoom_level: string;
	crouch_scale: string;
	health: string;
	shields: string;
	has_camo: boolean;
	has_overshield: boolean;
	frags: number;
	plasmas: number;
	selected_weapon_slot: number;
	biped_tag: string;
	actions: TickPlayerActions;
	weapons: TickWeaponV2[];
}

export interface PrettyPowerItemRow {
	spawn_id: number;
	status: string;
	held_by: string;
	pos_x: string;
	pos_y: string;
	pos_z: string;
	respawn_in_ticks: string;
}

export interface PrettyCTFFlagRow {
	team: number;
	pos_x: string;
	pos_y: string;
	pos_z: string;
	carrier: string;
	status: string;
}

export interface PrettyWeaponRow {
	slot: number;
	tag: string;
	object_id: number;
	ammo_mag: string;
	ammo_pack: string;
	charge: string;
	heat: string;
	reloading: boolean;
}

export interface PrettyLocalCard {
	local_index: number;
	fp_weapon: {
		state: FieldDisplay;
		animation_id: FieldDisplay;
		animation_tick: FieldDisplay;
	};
	observer_cam: {
		pos: FieldDisplay;
		aim: FieldDisplay;
		fov: FieldDisplay;
	};
	input: {
		left_stick: FieldDisplay;
		right_stick: FieldDisplay;
		buttons: FieldDisplay;
	};
	player_control: {
		desired_yaw: FieldDisplay;
		desired_pitch: FieldDisplay;
		zoom_level: FieldDisplay;
		aim_assist_target: FieldDisplay;
	};
}

export interface GameGlobalsTiles {
	map_loaded: FieldDisplay;
	active: FieldDisplay;
	game_loading_in_progress: FieldDisplay;
	precache_map_status: FieldDisplay;
	stored_global_random: FieldDisplay;
}

export interface TickPrettyVm {
	players: PrettyPlayerRow[];
	powerItems: PrettyPowerItemRow[];
	ctfFlags: PrettyCTFFlagRow[];
	gameGlobals: GameGlobalsTiles;
	locals: PrettyLocalCard[];
}

function fmtNum(n: number | null | undefined, digits = 4): string {
	if (n === null || n === undefined || !Number.isFinite(n)) return '—';
	if (Number.isInteger(n)) return String(n);
	return n.toFixed(digits);
}

function fmtVec3(v: Vec3 | null | undefined): string {
	if (!v) return '—';
	return `${fmtNum(v.x)}, ${fmtNum(v.y)}, ${fmtNum(v.z)}`;
}

function fmtVec2(v: { x: number; y: number } | null | undefined): string {
	if (!v) return '—';
	return `${fmtNum(v.x)}, ${fmtNum(v.y)}`;
}

function hex8(n: number): string {
	const u = n >>> 0;
	return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
}

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function fieldNum(value: number | null | undefined): FieldDisplay {
	if (value === null || value === undefined) return field('—', undefined, false);
	return field(fmtNum(value), String(value), true);
}

function fieldFlag(value: number | null | undefined): FieldDisplay {
	// game_globals fields are uint8 on the wire — 0/1 as flag-like but
	// other small ints (precache_map_status) read as real numbers. Render
	// '·' / '✓' for flag-asserted vs cleared and pass through any other.
	if (value === null || value === undefined) return field('—', undefined, false);
	if (value === 0) return field('·', '0', true);
	if (value === 1) return field('✓', '1', true);
	return field(String(value), String(value), true);
}

function fieldRandom(value: number | null | undefined): FieldDisplay {
	// stored_global_random is a u32 — surface both hex + decimal so the
	// reader can correlate against memory dumps.
	if (value === null || value === undefined) return field('—', undefined, false);
	return field(`${hex8(value)} (${value >>> 0})`, String(value), true);
}

function missingField(): FieldDisplay {
	return field('—', undefined, false);
}

function prettyPlayerRow(p: TickPlayerV2): PrettyPlayerRow {
	return {
		index: p.index,
		alive: p.alive,
		respawn_in_ticks: p.respawn_in_ticks === null ? '—' : String(p.respawn_in_ticks),
		pos_x: fmtNum(p.pos?.x),
		pos_y: fmtNum(p.pos?.y),
		pos_z: fmtNum(p.pos?.z),
		vel_x: fmtNum(p.vel?.x),
		vel_y: fmtNum(p.vel?.y),
		vel_z: fmtNum(p.vel?.z),
		aim_x: fmtNum(p.aim?.x),
		aim_y: fmtNum(p.aim?.y),
		aim_z: fmtNum(p.aim?.z),
		zoom_level: fmtNum(p.zoom_level),
		crouch_scale: fmtNum(p.crouch_scale),
		health: fmtNum(p.health),
		shields: fmtNum(p.shields),
		has_camo: p.has_camo,
		has_overshield: p.has_overshield,
		frags: p.frags,
		plasmas: p.plasmas,
		selected_weapon_slot: p.selected_weapon_slot,
		biped_tag: p.biped_tag || '—',
		actions: p.actions,
		weapons: p.weapons ?? []
	};
}

function prettyPowerItemRow(p: TickPowerItem): PrettyPowerItemRow {
	return {
		spawn_id: p.spawn_id,
		status: p.status,
		held_by: p.held_by === null ? '—' : String(p.held_by),
		pos_x: fmtNum(p.pos?.x ?? null),
		pos_y: fmtNum(p.pos?.y ?? null),
		pos_z: fmtNum(p.pos?.z ?? null),
		respawn_in_ticks: p.respawn_in_ticks === null ? '—' : String(p.respawn_in_ticks)
	};
}

function prettyCTFRow(f: TickCTFFlag): PrettyCTFFlagRow {
	return {
		team: f.team,
		pos_x: fmtNum(f.pos?.x),
		pos_y: fmtNum(f.pos?.y),
		pos_z: fmtNum(f.pos?.z),
		carrier: f.carrier === null ? '—' : String(f.carrier),
		status: f.status
	};
}

export function prettyWeaponRow(w: TickWeaponV2): PrettyWeaponRow {
	return {
		slot: w.slot,
		tag: w.tag || '—',
		object_id: w.object_id,
		ammo_mag: w.ammo_mag === null ? '—' : String(w.ammo_mag),
		ammo_pack: w.ammo_pack === null ? '—' : String(w.ammo_pack),
		charge: w.charge === null ? '—' : fmtNum(w.charge),
		heat: fmtNum(w.heat),
		reloading: w.reloading
	};
}

function gameGlobalsTiles(g: TickGameGlobals | null | undefined): GameGlobalsTiles {
	return {
		map_loaded: fieldFlag(g?.map_loaded),
		active: fieldFlag(g?.active),
		game_loading_in_progress: fieldFlag(g?.game_loading_in_progress),
		precache_map_status: fieldFlag(g?.precache_map_status),
		stored_global_random: fieldRandom(g?.stored_global_random)
	};
}

function prettyLocalCard(l: TickLocal): PrettyLocalCard {
	const fp = l.fp_weapon ?? null;
	const obs = l.observer_cam ?? null;
	const inp = l.input ?? null;
	const pc = l.player_control ?? null;

	return {
		local_index: l.local_index,
		fp_weapon: {
			state: fieldNum(fp?.state),
			animation_id: fieldNum(fp?.animation_id),
			animation_tick: fieldNum(fp?.animation_tick)
		},
		observer_cam: {
			pos: obs ? field(fmtVec3(obs.pos), JSON.stringify(obs.pos), true) : missingField(),
			aim: obs ? field(fmtVec3(obs.aim), JSON.stringify(obs.aim), true) : missingField(),
			fov: fieldNum(obs?.fov)
		},
		input: {
			left_stick: inp
				? field(fmtVec2(inp.left_stick), JSON.stringify(inp.left_stick), true)
				: missingField(),
			right_stick: inp
				? field(fmtVec2(inp.right_stick), JSON.stringify(inp.right_stick), true)
				: missingField(),
			buttons: inp
				? field(
						`a:${inp.buttons.a ? '✓' : '·'}  b:${inp.buttons.b ? '✓' : '·'}`,
						JSON.stringify(inp.buttons),
						true
					)
				: missingField()
		},
		player_control: {
			desired_yaw: fieldNum(pc?.desired_yaw),
			desired_pitch: fieldNum(pc?.desired_pitch),
			zoom_level: fieldNum(pc?.zoom_level),
			aim_assist_target:
				pc?.aim_assist_target == null
					? field('—', undefined, false)
					: fieldNum(pc.aim_assist_target)
		}
	};
}

export function buildTickPrettyVm(payload: TickPayloadV2 | null): TickPrettyVm {
	const players = payload?.players ?? [];
	const powerItems = payload?.power_items ?? [];
	const ctfFlags = payload?.ctf_flags ?? [];
	const locals = payload?.locals ?? [];

	return {
		players: players.map(prettyPlayerRow),
		powerItems: powerItems.map(prettyPowerItemRow),
		ctfFlags: ctfFlags.map(prettyCTFRow),
		gameGlobals: gameGlobalsTiles(payload?.game_globals ?? null),
		locals: locals.map(prettyLocalCard)
	};
}

// Active-action labels (e.g. "firing", "crouching") in stable display order.
// Empty array when no action is active — call site renders an em-dash.
export function activeActionLabels(actions: TickPlayerActions | null | undefined): string[] {
	if (!actions) return [];
	const out: string[] = [];
	if (actions.crouching) out.push('crouching');
	if (actions.jumping) out.push('jumping');
	if (actions.firing) out.push('firing');
	if (actions.flashlight) out.push('flashlight');
	if (actions.throwing_grenade) out.push('throwing_grenade');
	if (actions.meleeing) out.push('meleeing');
	if (actions.using) out.push('using');
	return out;
}
