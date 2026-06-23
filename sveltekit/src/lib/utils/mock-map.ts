// Per-map demo model: places mock players + items WITHIN a map's real
// structure-BSP world-bounds, so any extracted map (not just Blood Gulch) can be
// shown populated without a live game on that map. Used by the visualizer pages
// when `?map=<key>` is set (the sanctioned fallback when the live instance can't
// load that map). Pure / IO-free → unit-tested, and map-agnostic: every anchor
// is derived from the passed bounds, so it adapts to any map's scale (Blood
// Gulch ~126×145 wu vs Chill Out / Prisoner ~±13 wu).
//
// Bounds are set to the geometry's exact bounds, so the 2D background image + 3D
// camera framing align with the markers BY CONSTRUCTION — which is exactly the
// per-map "world-bounds auto-fit + coordinate alignment" we want to prove.

import { prettify, teamMeta } from '$lib/utils/overlay-view';
import type {
	VizModel,
	VizPlayer,
	VizItem,
	VizVehicle,
	VizSpawn,
	WorldBounds
} from '$lib/utils/visualizer-view';
import type { BspMesh } from '$lib/utils/game-geometry';
import { buildFloorplan, type FloorTri } from '$lib/utils/floorplan';
import { computeFloorBands } from '$lib/utils/floor-bands';

export interface BoundsLike {
	minX: number;
	maxX: number;
	minY: number;
	maxY: number;
	minZ: number;
	maxZ: number;
}

const RED_NAMES = ['Stewball', 'gravemind', 'noble_six', 'CmdrKeyes'];
const BLUE_NAMES = ['TheArbiter', 'TartarusX', 'Regret', 'flood_carrier'];

function mkItem(
	id: string,
	tag: string,
	kind: VizItem['kind'],
	x: number,
	y: number,
	z: number
): VizItem {
	return {
		id,
		tag,
		label: prettify(tag),
		kind,
		pos: { x, y, z },
		heldBy: null,
		respawning: false,
		respawnIn: null
	};
}

/**
 * Build a believable 4v4 Team Slayer VizModel placed inside `b` (a map's real
 * BSP world-bounds). Two bases sit ~18% / 82% along the map's longer horizontal
 * axis; each team clusters near its base with one player roaming toward mid.
 * `frame` (optional) rotates the clusters so the demo can animate.
 */
export function mockMapModel(b: BoundsLike, scenario: string, frame = 0): VizModel {
	const spanX = b.maxX - b.minX;
	const spanY = b.maxY - b.minY;
	const longX = spanX >= spanY;
	const longSpan = longX ? spanX : spanY;
	const cx = (b.minX + b.maxX) / 2;
	const cy = (b.minY + b.maxY) / 2;
	// Sit markers just above the floor (scaled to the map's vertical extent).
	const floor = b.minZ + Math.min(2, Math.max(0.3, (b.maxZ - b.minZ) * 0.12));

	const clampX = (v: number) => Math.max(b.minX, Math.min(b.maxX, v));
	const clampY = (v: number) => Math.max(b.minY, Math.min(b.maxY, v));

	const at = (f: number) =>
		longX ? { x: b.minX + spanX * f, y: cy } : { x: cx, y: b.minY + spanY * f };
	const baseA = at(0.18);
	const baseB = at(0.82);
	const mid = { x: cx, y: cy };
	const r = Math.max(0.5, longSpan * 0.08); // cluster radius scales with the map

	const players: VizPlayer[] = [];
	const place = (
		idx: number,
		name: string,
		team: number,
		base: { x: number; y: number },
		k: number,
		roam: boolean
	) => {
		let x: number, y: number;
		if (roam) {
			const t = 0.45;
			x = base.x + (mid.x - base.x) * t;
			y = base.y + (mid.y - base.y) * t;
		} else {
			const ang = (k / 4) * Math.PI * 2 + frame * 0.02;
			x = base.x + Math.cos(ang) * r;
			y = base.y + Math.sin(ang) * r;
		}
		x = clampX(x);
		y = clampY(y);
		const target = team === 0 ? baseB : baseA;
		// Screen-space heading (atan2(-ay, ax)) so Scene3D's facingAngle recovers
		// the world-plane direction — same convention buildVizModel uses.
		const heading = Math.atan2(-(target.y - y), target.x - x);
		players.push({
			index: idx,
			name,
			team,
			color: teamMeta(team).color,
			isLocal: team === 0 && k === 0,
			alive: true,
			respawnIn: null,
			health: 0.55 + 0.4 * ((idx % 3) / 2),
			shields: idx % 2 === 0 ? 1 : 0.4,
			hasOvershield: false,
			hasCamo: false,
			pos: { x, y, z: floor },
			heading
		});
	};
	for (let k = 0; k < 4; k++) place(k, RED_NAMES[k], 0, baseA, k, k === 0);
	for (let k = 0; k < 4; k++) place(4 + k, BLUE_NAMES[k], 1, baseB, k, k === 0);

	const items: VizItem[] = [
		mkItem('demo-rl', 'weapons\\rocket launcher\\rocket launcher', 'weapon', mid.x, mid.y, floor),
		mkItem(
			'demo-os',
			'powerups\\over shield\\over shield',
			'powerup',
			(baseA.x + mid.x) / 2,
			(baseA.y + mid.y) / 2,
			floor
		),
		mkItem(
			'demo-snipe',
			'weapons\\sniper rifle\\sniper rifle',
			'weapon',
			(baseB.x + mid.x) / 2,
			(baseB.y + mid.y) / 2,
			floor
		)
	];
	const vehicles: VizVehicle[] = [];
	// Two corner spawns anchor any auto-fit consumer to the full map extent, plus
	// a spawn under each player (reference layer; hidden unless toggled on).
	const spawns: VizSpawn[] = [
		{ pos: { x: b.minX, y: b.minY, z: floor }, team: 0, color: teamMeta(0).color },
		{ pos: { x: b.maxX, y: b.maxY, z: floor }, team: 1, color: teamMeta(1).color },
		...players.map((p) => ({ pos: p.pos, team: p.team, color: p.color }))
	];

	const bounds: WorldBounds = { ...b, valid: true, source: 'static' };
	return {
		mapName: prettify(scenario) || scenario,
		gametype: 'Team Slayer',
		variant: 'Team Slayer',
		phase: 'live',
		isTeamGame: true,
		players,
		items,
		vehicles,
		flags: [],
		spawns,
		projectiles: [],
		bounds,
		playerCount: players.length,
		placedCount: players.length,
		respawningItems: 0,
		hasGame: true,
		hasTick: true,
		hasScenario: true,
		hasObjects: false,
		hasPositions: true
	};
}

// =============================================================================
// Multi-floor staged mock (spectator-readability prototype)
// =============================================================================

/** Squared XY distance — used for farthest-point spreading + nearest-floor picks. */
function d2(a: { x: number; y: number }, b: { x: number; y: number }): number {
	const dx = a.x - b.x;
	const dy = a.y - b.y;
	return dx * dx + dy * dy;
}

/** Pick up to `n` floor points spread across `cands` by farthest-point selection
 *  (deterministic: seeds from the candidate nearest the group's XY centroid). */
function spreadPoints(cands: { x: number; y: number; z: number }[], n: number) {
	if (cands.length === 0) return [];
	const cx = cands.reduce((s, c) => s + c.x, 0) / cands.length;
	const cy = cands.reduce((s, c) => s + c.y, 0) / cands.length;
	let seedI = 0;
	let seedD = Infinity;
	for (let i = 0; i < cands.length; i++) {
		const d = d2(cands[i], { x: cx, y: cy });
		if (d < seedD) {
			seedD = d;
			seedI = i;
		}
	}
	const chosen = [cands[seedI]];
	while (chosen.length < Math.min(n, cands.length)) {
		let bestI = -1;
		let bestD = -1;
		for (let i = 0; i < cands.length; i++) {
			let nearest = Infinity;
			for (const c of chosen) {
				const d = d2(cands[i], c);
				if (d < nearest) nearest = d;
			}
			if (nearest > bestD) {
				bestD = nearest;
				bestI = i;
			}
		}
		if (bestI < 0) break;
		chosen.push(cands[bestI]);
	}
	return chosen;
}

const STAGED_NAMES = [
	'Stewball',
	'gravemind',
	'noble_six',
	'CmdrKeyes',
	'TheArbiter',
	'TartarusX',
	'Regret',
	'flood_carrier'
];

/**
 * Stage a 4v4 with players standing on REAL floor surfaces at different
 * elevations — the demo that makes every readability feature visible at once:
 * players on different floor bands (elevation shading), a cluster sharing one
 * room (occupancy reveal), and bodies separated by walls / floors (through-wall
 * markers + camera-occlusion). Floors + bands come from the same extracted mesh
 * the visualizer renders, so every body sits on geometry by construction.
 *
 * Distribution: one player per floor band, lowest→highest (guarantees every band
 * is populated), then the remaining players cluster into the lowest band's main
 * room. Teams alternate so each floor mixes red/blue.
 */
export function mockStagedModel(mesh: BspMesh, scenario: string, count = 8): VizModel {
	const { floors } = buildFloorplan(mesh);
	const bands = computeFloorBands(
		floors.map((f) => f.z),
		5
	);

	// Group floor triangles by band, keep each as a candidate standing point.
	const byBand = new Map<number, { x: number; y: number; z: number }[]>();
	for (const f of floors) {
		const c = {
			x: (f.pts[0].x + f.pts[1].x + f.pts[2].x) / 3,
			y: (f.pts[0].y + f.pts[1].y + f.pts[2].y) / 3,
			z: f.z
		};
		const b = bands.bandForZ(f.z);
		const arr = byBand.get(b);
		if (arr) arr.push(c);
		else byBand.set(b, [c]);
	}
	const populatedBands = [...byBand.keys()].sort((a, b) => a - b);

	// Build a target list: one slot per populated band (low→high), then pad the
	// rest into the lowest band as a clustered room.
	type Target = { x: number; y: number; z: number };
	const targets: Target[] = [];
	for (const b of populatedBands) {
		if (targets.length >= count) break;
		const cands = byBand.get(b)!;
		const pt = spreadPoints(cands, 1)[0];
		if (pt) targets.push(pt);
	}
	if (populatedBands.length > 0) {
		const lowest = byBand.get(populatedBands[0])!;
		const cluster = spreadPoints(lowest, Math.max(1, count - targets.length) + 1).slice(1);
		for (const pt of cluster) {
			if (targets.length >= count) break;
			targets.push(pt);
		}
	}
	// Final fallback: if a sparse mesh gave too few floor points, jitter around
	// the map centre so we still place `count` players.
	while (targets.length < count) {
		const cx = (mesh.bounds.minX + mesh.bounds.maxX) / 2;
		const cy = (mesh.bounds.minY + mesh.bounds.maxY) / 2;
		const a = (targets.length / count) * Math.PI * 2;
		const r = Math.max(0.5, (mesh.bounds.maxX - mesh.bounds.minX) * 0.12);
		targets.push({ x: cx + Math.cos(a) * r, y: cy + Math.sin(a) * r, z: mesh.bounds.minZ });
	}

	const cx = (mesh.bounds.minX + mesh.bounds.maxX) / 2;
	const cy = (mesh.bounds.minY + mesh.bounds.maxY) / 2;
	const lift = Math.min(1.2, Math.max(0.25, (mesh.bounds.maxZ - mesh.bounds.minZ) * 0.05));

	const players: VizPlayer[] = targets.slice(0, count).map((t, i) => {
		const team = i % 2; // alternate so each floor mixes teams
		// Face roughly toward the map centre (screen-space heading convention).
		const heading = Math.atan2(-(cy - t.y), cx - t.x);
		return {
			index: i,
			name: STAGED_NAMES[i % STAGED_NAMES.length],
			team,
			color: teamMeta(team).color,
			isLocal: i === 0,
			alive: true,
			respawnIn: null,
			health: 0.6 + 0.4 * (((i * 7) % 5) / 4),
			shields: i % 3 === 0 ? 0.5 : 1,
			hasOvershield: i === 0,
			hasCamo: false,
			pos: { x: t.x, y: t.y, z: t.z + lift },
			heading
		};
	});

	// A few reference power items on the primary (lowest) floor.
	const items: VizItem[] = [];
	if (targets.length > 0) {
		const lo = targets[0];
		items.push(
			mkItem('demo-os', 'powerups\\over shield\\over shield', 'powerup', lo.x, lo.y, lo.z + lift)
		);
	}

	const spawns: VizSpawn[] = [
		{
			pos: { x: mesh.bounds.minX, y: mesh.bounds.minY, z: mesh.bounds.minZ },
			team: 0,
			color: teamMeta(0).color
		},
		{
			pos: { x: mesh.bounds.maxX, y: mesh.bounds.maxY, z: mesh.bounds.maxZ },
			team: 1,
			color: teamMeta(1).color
		},
		...players.map((p) => ({ pos: p.pos, team: p.team, color: p.color }))
	];

	const bounds: WorldBounds = { ...mesh.bounds, valid: true, source: 'static' };
	return {
		mapName: prettify(scenario) || scenario,
		gametype: 'Team Slayer',
		variant: 'Team Slayer',
		phase: 'live',
		isTeamGame: true,
		players,
		items,
		vehicles: [] as VizVehicle[],
		flags: [],
		spawns,
		projectiles: [],
		bounds,
		playerCount: players.length,
		placedCount: players.length,
		respawningItems: 0,
		hasGame: true,
		hasTick: true,
		hasScenario: true,
		hasObjects: false,
		hasPositions: true
	};
}

/** Floor-triangle helper re-export for the staged mock's consumers (kept here so
 *  the mock and the renderer share one floorplan definition). */
export type { FloorTri };
