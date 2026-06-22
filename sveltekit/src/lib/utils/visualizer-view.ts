// Pure view-model builder for the live top-down game-state VISUALIZER (the M11
// minimap render, now unblocked by the verified spatial offsets — player / item
// / vehicle XYZ on the wire). Takes the raw v2 `game` + `tick` + `scenario` (+
// optional `objects`) payloads off the same WS feed the OBS overlays consume and
// projects them into a render-ready, source-agnostic model.
//
// Kept IO-free (no socket, no DOM) so the projection + classification math —
// the part that has to be CORRECT — unit-tests without a browser. The component
// (TopDownMap.svelte) only does the SVG drawing.
//
// Coordinate system (Halo: CE / H2, RUNTIME-VERIFIED biped pos at +0x0C..0x14):
//   - X / Y are the horizontal ground plane; Z is up.
//   - We project X→screen-x and Y→screen-y, flipping Y (world +Y is "north",
//     SVG y grows down). Z is carried for height cues but not used for the box.
//
// World bounds: the wire does NOT (yet) expose an explicit map bounding box, so
// we AUTO-FIT. Static scenario spawns (player + power-item placements) span the
// whole playable map and don't move, giving a rock-steady box; live entity
// positions are unioned in so nothing ever clips. A live game with the scenario
// class subscribed therefore renders a stable, correctly-scaled map without any
// per-map traced asset — which is also what makes this a cross-map / cross-game
// visual validation of the position offsets.

import type {
	GamePayload,
	ObjectsPayload,
	ScenarioPayload,
	TickPayloadV2,
	Vec2,
	Vec3
} from '$lib/types/scraper-v2';
import { prettify, teamMeta } from '$lib/utils/overlay-view';

// =============================================================================
// Entity view shapes
// =============================================================================

export interface VizPlayer {
	index: number;
	name: string;
	team: number;
	color: string;
	isLocal: boolean;
	alive: boolean;
	respawnIn: number | null;
	/** 0..1 engine body_vitality fraction (clamped). */
	health: number;
	/** 0..1 (clamped; over-full carried by hasOvershield). */
	shields: number;
	hasOvershield: boolean;
	hasCamo: boolean;
	pos: Vec3;
	/** Screen-space heading in radians from the aim vector (0 = +x / right,
	 * increasing clockwise on screen), or null when aim is ~zero. Accounts for
	 * the projection's Y flip; scale-independent. */
	heading: number | null;
}

export interface VizItem {
	id: string;
	tag: string;
	label: string;
	kind: TagClass;
	pos: Vec3;
	/** Player index holding it, else null (world / respawning). */
	heldBy: number | null;
	/** Respawning (no world position) — surfaced as a count, not a dot. */
	respawning: boolean;
	respawnIn: number | null;
}

export interface VizVehicle {
	id: string;
	tag: string;
	label: string;
	pos: Vec3;
	/** Has an occupant (owner_unit set). */
	occupied: boolean;
}

export interface VizFlag {
	team: number;
	color: string;
	label: string;
	pos: Vec3;
	status: string;
	carrier: number | null;
}

export interface VizSpawn {
	pos: Vec3;
	team: number;
	color: string;
}

export interface VizProjectile {
	id: string;
	tag: string;
	pos: Vec3;
}

// =============================================================================
// Bounds + projection
// =============================================================================

export interface WorldBounds {
	minX: number;
	maxX: number;
	minY: number;
	maxY: number;
	minZ: number;
	maxZ: number;
	valid: boolean;
	/** Where the box came from — `static` (scenario spawns, stable) is preferred;
	 * `dynamic` (live entities only) jitters until the scenario class arrives. */
	source: 'static' | 'dynamic' | 'none';
}

/** Minimum world span enforced on each axis so a single clustered frame (or a
 * not-yet-loaded scenario) doesn't divide-by-zero or zoom to infinity. */
const MIN_SPAN = 10;

export function computeBounds(positions: Vec3[], source: WorldBounds['source']): WorldBounds {
	const empty: WorldBounds = {
		minX: 0,
		maxX: 0,
		minY: 0,
		maxY: 0,
		minZ: 0,
		maxZ: 0,
		valid: false,
		source: 'none'
	};
	const pts = positions.filter((p) => p && Number.isFinite(p.x) && Number.isFinite(p.y));
	if (pts.length === 0) return empty;

	let minX = Infinity,
		maxX = -Infinity,
		minY = Infinity,
		maxY = -Infinity,
		minZ = Infinity,
		maxZ = -Infinity;
	for (const p of pts) {
		if (p.x < minX) minX = p.x;
		if (p.x > maxX) maxX = p.x;
		if (p.y < minY) minY = p.y;
		if (p.y > maxY) maxY = p.y;
		const z = Number.isFinite(p.z) ? p.z : 0;
		if (z < minZ) minZ = z;
		if (z > maxZ) maxZ = z;
	}

	// Enforce a minimum span on each horizontal axis, centred on the data.
	if (maxX - minX < MIN_SPAN) {
		const c = (minX + maxX) / 2;
		minX = c - MIN_SPAN / 2;
		maxX = c + MIN_SPAN / 2;
	}
	if (maxY - minY < MIN_SPAN) {
		const c = (minY + maxY) / 2;
		minY = c - MIN_SPAN / 2;
		maxY = c + MIN_SPAN / 2;
	}
	if (!Number.isFinite(minZ)) {
		minZ = 0;
		maxZ = 0;
	}
	return { minX, maxX, minY, maxY, minZ, maxZ, valid: true, source };
}

export interface MapProjection {
	valid: boolean;
	bounds: WorldBounds;
	/** Pixels per world unit (uniform, aspect-preserving). */
	scale: number;
	/** World position → viewport point (origin top-left, y down). */
	project(pos: Vec3): Vec2;
}

/**
 * Build an aspect-preserving projection that fits `bounds` into a
 * `width`×`height` viewport with `pad` px of margin, centring the content and
 * flipping Y (world +Y points up; SVG y grows down).
 */
export function makeProjection(
	bounds: WorldBounds,
	width: number,
	height: number,
	pad: number
): MapProjection {
	const spanX = Math.max(MIN_SPAN, bounds.maxX - bounds.minX);
	const spanY = Math.max(MIN_SPAN, bounds.maxY - bounds.minY);
	const availW = Math.max(1, width - 2 * pad);
	const availH = Math.max(1, height - 2 * pad);
	const scale = Math.min(availW / spanX, availH / spanY);
	const contentW = spanX * scale;
	const contentH = spanY * scale;
	const offX = pad + (availW - contentW) / 2;
	const offY = pad + (availH - contentH) / 2;
	return {
		valid: bounds.valid,
		bounds,
		scale,
		project(pos: Vec3): Vec2 {
			const x = Number.isFinite(pos?.x) ? pos.x : bounds.minX;
			const y = Number.isFinite(pos?.y) ? pos.y : bounds.minY;
			return {
				x: offX + (x - bounds.minX) * scale,
				// Flip Y: world maxY lands at the top of the content band.
				y: offY + (bounds.maxY - y) * scale
			};
		}
	};
}

// =============================================================================
// Tag classification (cross-game: tag prefix first, numeric type fallback)
// =============================================================================

export type TagClass =
	| 'vehicle'
	| 'weapon'
	| 'equipment'
	| 'powerup'
	| 'biped'
	| 'projectile'
	| 'scenery'
	| 'other';

/** Halo engine object-type enum → class, used as a fallback when a tag string is
 * ambiguous (0 biped, 1 vehicle, 2 weapon, 3 equipment, 5 projectile, 6
 * scenery). */
function classFromType(type: number | undefined): TagClass | null {
	switch (type) {
		case 0:
			return 'biped';
		case 1:
			return 'vehicle';
		case 2:
			return 'weapon';
		case 3:
			return 'equipment';
		case 5:
			return 'projectile';
		case 6:
			return 'scenery';
		default:
			return null;
	}
}

export function classifyTag(tag: string | null | undefined, type?: number): TagClass {
	const t = (tag ?? '').toLowerCase();
	// Tag paths use backslash separators ("vehicles\\warthog\\warthog").
	const head = t.split(/[\\/]/)[0];
	if (head === 'vehicles') return 'vehicle';
	if (
		head === 'powerups' ||
		t.includes('powerup') ||
		t.includes('over shield') ||
		t.includes('camouflage')
	)
		return 'powerup';
	if (head === 'equipment') return 'equipment';
	if (head === 'weapons') return 'weapon';
	if (head === 'characters' || t.includes('biped')) return 'biped';
	if (head === 'scenery') return 'scenery';
	if (t.includes('projectile') || t.includes('grenade')) return 'projectile';
	return classFromType(type) ?? 'other';
}

function clamp01(n: number): number {
	if (!Number.isFinite(n)) return 0;
	return n < 0 ? 0 : n > 1 ? 1 : n;
}

function headingFromAim(aim: Vec3 | undefined): number | null {
	if (!aim) return null;
	const ax = Number.isFinite(aim.x) ? aim.x : 0;
	const ay = Number.isFinite(aim.y) ? aim.y : 0;
	if (Math.abs(ax) < 1e-6 && Math.abs(ay) < 1e-6) return null;
	// Y flips under projection, so the screen-space direction of world (ax, ay)
	// is (ax, -ay).
	return Math.atan2(-ay, ax);
}

// =============================================================================
// Model
// =============================================================================

export interface VizModel {
	mapName: string;
	gametype: string;
	variant: string;
	phase: string;
	isTeamGame: boolean;

	players: VizPlayer[];
	items: VizItem[];
	vehicles: VizVehicle[];
	flags: VizFlag[];
	spawns: VizSpawn[];
	projectiles: VizProjectile[];

	bounds: WorldBounds;

	/** Named roster size (every console's players), independent of placement. */
	playerCount: number;
	/** Players actually placed on the map (have a tick position). */
	placedCount: number;
	/** Power items currently respawning (not on the map). */
	respawningItems: number;

	// Capability flags so the UI can show clean placeholders for what the feed
	// isn't currently exposing rather than asserting empty == "none".
	hasGame: boolean;
	hasTick: boolean;
	hasScenario: boolean;
	hasObjects: boolean;
	hasPositions: boolean;
}

const EMPTY_BOUNDS: WorldBounds = {
	minX: 0,
	maxX: 0,
	minY: 0,
	maxY: 0,
	minZ: 0,
	maxZ: 0,
	valid: false,
	source: 'none'
};

/**
 * buildVizModel joins the roster (identity + team) with the live tick
 * (position / alive / health / shields), folds in scenario spawns, tracked
 * power items, and — when the objects class is subscribed — world vehicles /
 * dropped items / projectiles, then auto-fits a stable world bounding box.
 * Every layer degrades independently: missing a class just empties that layer
 * and flips its capability flag.
 */
export function buildVizModel(
	game: GamePayload | null | undefined,
	tick: TickPayloadV2 | null | undefined,
	scenario: ScenarioPayload | null | undefined,
	objects: ObjectsPayload | null | undefined
): VizModel {
	const hasGame = !!game && Array.isArray(game.players);
	const hasTick = !!tick && Array.isArray(tick.players);
	const hasScenario = !!scenario;
	const hasObjects = !!objects && Array.isArray(objects.objects);

	// Roster identity keyed by player index.
	const roster = new Map<number, GamePayload['players'][number]>();
	if (hasGame) for (const p of game!.players) roster.set(p.index, p);

	const namedCount = hasGame
		? game!.players.filter((p) => (p.name ?? '').trim().length > 0).length
		: 0;

	// --- Players (placed = has a tick position) ---
	const players: VizPlayer[] = [];
	if (hasTick) {
		for (const tp of tick!.players) {
			const r = roster.get(tp.index);
			const team = r?.team ?? 0;
			players.push({
				index: tp.index,
				name: (r?.name ?? '').trim() || `Player ${tp.index}`,
				team,
				color: teamMeta(team).color,
				isLocal: r?.is_local === true,
				alive: tp.alive,
				respawnIn: tp.respawn_in_ticks,
				health: clamp01(tp.health),
				shields: clamp01(tp.shields),
				hasOvershield: tp.has_overshield === true,
				hasCamo: tp.has_camo === true,
				pos: tp.pos,
				heading: headingFromAim(tp.aim)
			});
		}
	}

	// --- Power items (scenario placement → label; tick → live status) ---
	const spawnTagById = new Map<number, string>();
	if (hasScenario)
		for (const s of scenario!.power_item_spawns ?? []) spawnTagById.set(s.spawn_id, s.tag);

	const items: VizItem[] = [];
	let respawningItems = 0;
	if (hasTick) {
		for (const pi of tick!.power_items ?? []) {
			const tag = spawnTagById.get(pi.spawn_id) ?? '';
			if (pi.status === 'respawning' || !pi.pos) {
				respawningItems += 1;
				continue;
			}
			items.push({
				id: `item-${pi.spawn_id}`,
				tag,
				label: prettify(tag) || `Item ${pi.spawn_id}`,
				kind: classifyTag(tag) === 'other' ? 'powerup' : classifyTag(tag),
				pos: pi.pos,
				heldBy: pi.held_by,
				respawning: false,
				respawnIn: pi.respawn_in_ticks
			});
		}
	}

	// --- Vehicles + extra items + projectiles from the objects class ---
	const vehicles: VizVehicle[] = [];
	const projectiles: VizProjectile[] = [];
	if (hasObjects) {
		for (const o of objects!.objects) {
			const kind = classifyTag(o.tag, o.type);
			if (kind === 'vehicle') {
				vehicles.push({
					id: `veh-${o.object_id}`,
					tag: o.tag,
					label: prettify(o.tag) || `Vehicle ${o.object_id}`,
					pos: o.pos,
					occupied: o.owner_unit != null
				});
			} else if (kind === 'weapon' || kind === 'equipment' || kind === 'powerup') {
				// Only surface DROPPED world items here (no owner); tracked power
				// items already came from the tick layer.
				if (o.owner_unit == null && o.owner_object == null) {
					items.push({
						id: `obj-${o.object_id}`,
						tag: o.tag,
						label: prettify(o.tag) || `Item ${o.object_id}`,
						kind,
						pos: o.pos,
						heldBy: null,
						respawning: false,
						respawnIn: null
					});
				}
			}
		}
		for (const pr of objects!.projectiles ?? []) {
			projectiles.push({ id: `proj-${pr.object_id}`, tag: pr.tag, pos: pr.pos });
		}
	}

	// --- CTF flags ---
	const flags: VizFlag[] = [];
	if (hasTick) {
		for (const f of tick!.ctf_flags ?? []) {
			flags.push({
				team: f.team,
				color: teamMeta(f.team).color,
				label: `${teamMeta(f.team).name} flag`,
				pos: f.pos,
				status: f.status,
				carrier: f.carrier
			});
		}
	}

	// --- Static spawns (reference layer + stable bounds anchor) ---
	const spawns: VizSpawn[] = [];
	if (hasScenario) {
		for (const s of scenario!.player_spawns ?? []) {
			spawns.push({ pos: s.pos, team: s.team_index, color: teamMeta(s.team_index).color });
		}
	}

	// --- Auto-fit bounds: prefer static spawns (stable); always union live
	//     entities so nothing clips. Projectiles excluded — a stray rocket
	//     shouldn't blow up the box. ---
	const staticPts: Vec3[] = [];
	if (hasScenario) {
		for (const s of scenario!.player_spawns ?? []) staticPts.push(s.pos);
		for (const s of scenario!.power_item_spawns ?? []) staticPts.push(s.pos);
	}
	const dynamicPts: Vec3[] = [
		...players.map((p) => p.pos),
		...items.map((i) => i.pos),
		...vehicles.map((v) => v.pos),
		...flags.map((f) => f.pos)
	];
	const hasPositions = dynamicPts.length > 0 || staticPts.length > 0;
	const source: WorldBounds['source'] =
		staticPts.length >= 2 ? 'static' : dynamicPts.length > 0 ? 'dynamic' : 'none';
	const bounds =
		source === 'none' ? EMPTY_BOUNDS : computeBounds([...staticPts, ...dynamicPts], source);

	const cfg = game?.config ?? null;
	const isTeamGame = cfg ? cfg.is_team_game === true : new Set(players.map((p) => p.team)).size > 1;

	return {
		mapName: prettify(scenario?.map) || (hasScenario ? 'Unknown' : ''),
		gametype: prettify(cfg?.variant_name || cfg?.gametype),
		variant: cfg?.variant_name ?? '',
		phase: game?.phase ?? 'idle',
		isTeamGame,
		players,
		items,
		vehicles,
		flags,
		spawns,
		projectiles,
		bounds,
		playerCount: namedCount || players.length,
		placedCount: players.length,
		respawningItems,
		hasGame,
		hasTick,
		hasScenario,
		hasObjects,
		hasPositions
	};
}

/** Health-bar / ring colour ramp: green (full) → amber → red (low). */
export function healthColor(h: number): string {
	const t = clamp01(h);
	if (t >= 0.5) {
		// green → amber over [1, 0.5]
		const k = (t - 0.5) / 0.5; // 1 at full, 0 at half
		return mix('#e0a32e', '#36b55a', k);
	}
	// amber → red over [0.5, 0]
	const k = t / 0.5; // 1 at half, 0 empty
	return mix('#e3413f', '#e0a32e', k);
}

function mix(lo: string, hi: string, k: number): string {
	const a = hexToRgb(lo);
	const b = hexToRgb(hi);
	const t = clamp01(k);
	const r = Math.round(a.r + (b.r - a.r) * t);
	const g = Math.round(a.g + (b.g - a.g) * t);
	const bl = Math.round(a.b + (b.b - a.b) * t);
	return `rgb(${r}, ${g}, ${bl})`;
}

function hexToRgb(hex: string): { r: number; g: number; b: number } {
	const h = hex.replace('#', '');
	return {
		r: parseInt(h.slice(0, 2), 16),
		g: parseInt(h.slice(2, 4), 16),
		b: parseInt(h.slice(4, 6), 16)
	};
}
