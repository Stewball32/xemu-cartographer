// Pretty-view VM for the Objects debug tab — formats the ObjectsPayload into
// row arrays suitable for DataTable rendering. Both arrays are always present
// (empty when the payload is null); the DataTable renders its emptyMessage in
// that case.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type { ObjectsPayload, Projectile, WorldObject } from '$lib/types/scraper-v2';

export interface ObjectsPrettyVm {
	objectCount: number;
	projectileCount: number;
	objects: ObjectRow[];
	projectiles: ProjectileRow[];
}

/** Flattened row shape consumed by DataTable. Vec3 components are split into
 * per-axis cells so each axis can sort independently and the tabular layout
 * stays compact. Hex-formatted scalars are pre-rendered to strings; the raw
 * numeric values are kept as `*_raw` so sortAccessors order them numerically
 * (lex sort would put 0x1F0 after 0x1FF).
 */
export interface ObjectRow {
	object_id: string;
	object_id_raw: number;
	tag: string;
	type: number;
	flags: string;
	flags_raw: number;
	pos_x: number;
	pos_y: number;
	pos_z: number;
	ang_vel_x: number;
	ang_vel_y: number;
	ang_vel_z: number;
	time_existing: number;
	owner_unit: string;
	owner_unit_raw: number | null;
	owner_object: string;
	owner_object_raw: number | null;
	ultimate_parent: string;
	ultimate_parent_raw: number;
}

export interface ProjectileRow {
	object_id: string;
	object_id_raw: number;
	tag: string;
	pos_x: number;
	pos_y: number;
	pos_z: number;
	flags: string;
	flags_raw: number;
	action: number;
	detonation_timer: number;
	distance_traveled: number;
}

const SENTINEL_NULL = 0xffffffff;

function hex8(n: number): string {
	// Force unsigned 32-bit width — IDs and bitfields are u32 on the wire;
	// JS bitwise operators sign-extend, so we go via >>> 0.
	const u = n >>> 0;
	return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
}

/** Backend nulls owner refs equal to 0xFFFFFFFF, but a stray non-null value
 * matching the sentinel should still render as '—' so the UI never shows
 * the magic number. */
function refDisplay(n: number | null): { display: string; raw: number | null } {
	if (n === null) return { display: '—', raw: null };
	const u = n >>> 0;
	if (u === SENTINEL_NULL) return { display: '—', raw: null };
	return { display: hex8(u), raw: u };
}

function objectRow(o: WorldObject): ObjectRow {
	const ou = refDisplay(o.owner_unit);
	const oo = refDisplay(o.owner_object);
	return {
		object_id: hex8(o.object_id),
		object_id_raw: o.object_id >>> 0,
		tag: o.tag || '—',
		type: o.type,
		flags: hex8(o.flags),
		flags_raw: o.flags >>> 0,
		pos_x: o.pos.x,
		pos_y: o.pos.y,
		pos_z: o.pos.z,
		ang_vel_x: o.ang_vel.x,
		ang_vel_y: o.ang_vel.y,
		ang_vel_z: o.ang_vel.z,
		time_existing: o.time_existing,
		owner_unit: ou.display,
		owner_unit_raw: ou.raw,
		owner_object: oo.display,
		owner_object_raw: oo.raw,
		ultimate_parent: hex8(o.ultimate_parent),
		ultimate_parent_raw: o.ultimate_parent >>> 0
	};
}

function projectileRow(p: Projectile): ProjectileRow {
	return {
		object_id: hex8(p.object_id),
		object_id_raw: p.object_id >>> 0,
		tag: p.tag || '—',
		pos_x: p.pos.x,
		pos_y: p.pos.y,
		pos_z: p.pos.z,
		flags: hex8(p.flags),
		flags_raw: p.flags >>> 0,
		action: p.action,
		detonation_timer: p.detonation_timer,
		distance_traveled: p.distance_traveled
	};
}

export function buildObjectsPrettyVm(payload: ObjectsPayload | null): ObjectsPrettyVm {
	const objects = payload?.objects ?? [];
	const projectiles = payload?.projectiles ?? [];
	return {
		objectCount: objects.length,
		projectileCount: projectiles.length,
		objects: objects.map(objectRow),
		projectiles: projectiles.map(projectileRow)
	};
}
