// Pretty-view VM for the Debug envelope tab. Mirrors xbox-vm.ts: pure TS
// formatters that project the DebugPayload into per-section descriptors so
// the Pretty view always renders every expected field, using '—' for
// missing and a distinct marker for fields that are explicitly null on
// the wire (nullable members of DebugPlayer).
//
// Reactivity is wired at the call site via $derived.by().

import type {
	DebugBone,
	DebugPayload,
	DebugPlayer,
	DebugPlayerExtended,
	DebugSphere,
	DebugUpdateQueue
} from '$lib/types/scraper-v2';

/** Display state for one tile/field. `present` toggles StatTile's status dot
 * (success-green vs. greyed-out), `raw` is surfaced via the tile `title`
 * attribute so a hover reveals the underlying wire value. */
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

export interface ExtendedVm {
	/** 'present' | 'null' | 'missing' — the four `extended`/`bones`/
	 * `update_queue`/`raw` members of DebugPlayer are nullable, and we
	 * render null distinctly from missing so a Pretty-view reader can
	 * tell "scraper saw the field and it was null" apart from
	 * "scraper hasn't observed this field yet". */
	state: NullableState;
	legs_pitch: FieldDisplay;
	legs_yaw: FieldDisplay;
	legs_roll: FieldDisplay;
	airborne: FieldDisplay;
	airborne_ticks: FieldDisplay;
	idle_ticks: FieldDisplay;
	state_flags: FieldDisplay;
	stunned: FieldDisplay;
	aim_assist_sphere: FieldDisplay;
}

export interface UpdateQueueVm {
	state: NullableState;
	crouch: FieldDisplay;
	jump: FieldDisplay;
	fire: FieldDisplay;
	desired_yaw: FieldDisplay;
	desired_pitch: FieldDisplay;
}

export interface BoneRow {
	index: number;
	pos_x: FieldDisplay;
	pos_y: FieldDisplay;
	pos_z: FieldDisplay;
}

export interface BonesVm {
	state: NullableState;
	count: number;
	rows: BoneRow[];
}

export interface RawVm {
	state: NullableState;
	entries: Array<[string, unknown]>;
}

export interface PlayerVm {
	index: number;
	extended: ExtendedVm;
	bones: BonesVm;
	update_queue: UpdateQueueVm;
	raw: RawVm;
}

export interface DebugPrettyVm {
	players: {
		count: number;
		rows: PlayerVm[];
	};
}

export type NullableState = 'present' | 'null' | 'missing';

// =============================================================================
// formatters
// =============================================================================

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function missing(): FieldDisplay {
	return field('—', undefined, false);
}

function hex8(n: number): string {
	// Force unsigned 32-bit width — bitfields are u32 on the wire; JS bitwise
	// operators sign-extend, so we go via >>> 0.
	const u = n >>> 0;
	return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
}

function fmtFloat(n: number): string {
	if (!Number.isFinite(n)) return String(n);
	return n.toFixed(4);
}

function fmtFloatField(n: number | undefined): FieldDisplay {
	if (n === undefined) return missing();
	return field(fmtFloat(n), String(n), true);
}

function fmtIntField(n: number | undefined): FieldDisplay {
	if (n === undefined) return missing();
	return field(String(n), String(n), true);
}

function fmtBoolField(b: boolean | undefined): FieldDisplay {
	if (b === undefined) return missing();
	return field(b ? 'true' : 'false', String(b), true);
}

function fmtHexField(n: number | undefined): FieldDisplay {
	if (n === undefined) return missing();
	return field(`${hex8(n)} (${n >>> 0})`, String(n), true);
}

function fmtSphere(s: DebugSphere | undefined | null): FieldDisplay {
	if (s === undefined) return missing();
	if (s === null) return field('null', 'null', false);
	const raw = JSON.stringify(s);
	return field(
		`r=${fmtFloat(s.radius)} @ (${fmtFloat(s.center.x)}, ${fmtFloat(s.center.y)}, ${fmtFloat(s.center.z)})`,
		raw,
		true
	);
}

function nullableState(value: unknown): NullableState {
	if (value === undefined) return 'missing';
	if (value === null) return 'null';
	return 'present';
}

function buildExtendedVm(ext: DebugPlayerExtended | undefined | null): ExtendedVm {
	return {
		state: nullableState(ext),
		legs_pitch: fmtFloatField(ext?.legs?.pitch),
		legs_yaw: fmtFloatField(ext?.legs?.yaw),
		legs_roll: fmtFloatField(ext?.legs?.roll),
		airborne: fmtBoolField(ext?.airborne),
		airborne_ticks: fmtIntField(ext?.airborne_ticks),
		idle_ticks: fmtIntField(ext?.idle_ticks),
		state_flags: fmtHexField(ext?.state_flags),
		stunned: fmtIntField(ext?.stunned),
		aim_assist_sphere: fmtSphere(ext?.aim_assist_sphere)
	};
}

function buildUpdateQueueVm(q: DebugUpdateQueue | undefined | null): UpdateQueueVm {
	return {
		state: nullableState(q),
		crouch: fmtBoolField(q?.buttons?.crouch),
		jump: fmtBoolField(q?.buttons?.jump),
		fire: fmtBoolField(q?.buttons?.fire),
		desired_yaw: fmtFloatField(q?.desired_yaw),
		desired_pitch: fmtFloatField(q?.desired_pitch)
	};
}

function buildBonesVm(bones: DebugBone[] | undefined | null): BonesVm {
	const state = nullableState(bones);
	if (!bones) return { state, count: 0, rows: [] };
	const rows: BoneRow[] = bones.map((b) => ({
		index: b.index,
		pos_x: fmtFloatField(b.pos?.x),
		pos_y: fmtFloatField(b.pos?.y),
		pos_z: fmtFloatField(b.pos?.z)
	}));
	return { state, count: rows.length, rows };
}

function buildRawVm(raw: Record<string, unknown> | undefined | null): RawVm {
	const state = nullableState(raw);
	if (!raw) return { state, entries: [] };
	const entries = Object.entries(raw);
	entries.sort(([a], [b]) => a.localeCompare(b));
	return { state, entries };
}

function buildPlayerVm(p: DebugPlayer): PlayerVm {
	return {
		index: p.index,
		extended: buildExtendedVm(p.extended),
		bones: buildBonesVm(p.bones),
		update_queue: buildUpdateQueueVm(p.update_queue),
		raw: buildRawVm(p.raw)
	};
}

export function buildDebugPrettyVm(payload: DebugPayload | null): DebugPrettyVm {
	const players = payload?.players ?? [];
	return {
		players: {
			count: players.length,
			rows: players.map(buildPlayerVm)
		}
	};
}
