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
