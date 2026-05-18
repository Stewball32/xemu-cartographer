// Pretty-view VM for the Scenario debug tab — formats the ScenarioPayload
// into section records that mirror the envelope's JSON shape (top-level
// scalars + fog + memory_regions + the three list/map sections). Every
// scalar field is always present; payloads missing a field render as '—'
// so the Pretty view exposes the expected envelope structure even before
// the first read.
//
// Pure TS; reactivity is wired at the call site via $derived.by().

import type {
	ScenarioFog,
	ScenarioMemoryRegion,
	ScenarioObjectType,
	ScenarioPayload,
	ScenarioPlayerSpawn,
	ScenarioPowerItemSpawn,
	ScenarioTagDef
} from '$lib/types/scraper-v2';

// One tile's display state + the raw value (passed through StatTile's
// `title` prop so hovering a formatted tile reveals the underlying
// wire value).
export interface FieldDisplay {
	display: string;
	raw: string;
	present: boolean;
}

export interface ScenarioPrettyVm {
	hasPayload: boolean;
	topLevel: {
		map: FieldDisplay;
		game_difficulty: FieldDisplay;
	};
	fog: {
		present: boolean;
		color: FieldDisplay;
		max_density: FieldDisplay;
		atmo_min_dist: FieldDisplay;
		atmo_max_dist: FieldDisplay;
	};
	memoryRegions: {
		present: boolean;
		game_state: MemoryRegionVm;
		tag_cache: MemoryRegionVm;
		texture_cache: MemoryRegionVm;
		sound_cache: MemoryRegionVm;
	};
	objectTypes: ScenarioObjectType[];
	playerSpawns: ScenarioPlayerSpawn[];
	powerItemSpawns: ScenarioPowerItemSpawn[];
	tagDefs: TagDefEntry[];
}

export interface MemoryRegionVm {
	base: FieldDisplay;
	size: FieldDisplay;
}

/** Single entry of the keyed tag_defs map, paired with its tag key so the
 * Pretty view can render the discriminated union (`kind: 'weapon' | 'biped'`)
 * grouped by tag string. Sorted alphabetically by tag for stable order. */
export interface TagDefEntry {
	tag: string;
	def: ScenarioTagDef;
}

function hex8(n: number): string {
	// Force unsigned 32-bit width — pointers / sizes come across as u32 on
	// the wire; JS bitwise operators sign-extend so we go via >>> 0.
	const u = n >>> 0;
	return '0x' + u.toString(16).toUpperCase().padStart(8, '0');
}

function formatBytes(bytes: number): string {
	if (!Number.isFinite(bytes) || bytes < 0) return String(bytes);
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MiB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GiB`;
}

function field(display: string, raw: string | undefined, present: boolean): FieldDisplay {
	return { display, raw: raw ?? '—', present };
}

function fieldStr(value: string | undefined): FieldDisplay {
	const v = value ?? '';
	return field(v || '—', v || undefined, v.length > 0);
}

function fieldNum(value: number | undefined, render: (n: number) => string): FieldDisplay {
	if (value === undefined || value === null || !Number.isFinite(value)) {
		return field('—', value === undefined ? undefined : String(value), false);
	}
	return field(render(value), String(value), true);
}

function buildMemoryRegion(region: ScenarioMemoryRegion | undefined | null): MemoryRegionVm {
	return {
		base: fieldNum(region?.base, (n) => `${hex8(n)} (${n >>> 0})`),
		size: fieldNum(region?.size, (n) => `${formatBytes(n)} (${n})`)
	};
}

function formatColor(color: ScenarioFog['color'] | undefined | null): FieldDisplay {
	if (!color) return field('—', undefined, false);
	const { r, g, b } = color;
	const okR = Number.isFinite(r);
	const okG = Number.isFinite(g);
	const okB = Number.isFinite(b);
	if (!okR || !okG || !okB) return field('—', JSON.stringify(color), false);
	const display = `rgb(${r.toFixed(3)}, ${g.toFixed(3)}, ${b.toFixed(3)})`;
	return field(display, JSON.stringify(color), true);
}

function sortedTagDefs(tagDefs: Record<string, ScenarioTagDef> | undefined): TagDefEntry[] {
	if (!tagDefs) return [];
	return Object.entries(tagDefs)
		.map(([tag, def]) => ({ tag, def }))
		.sort((a, b) => a.tag.localeCompare(b.tag));
}

const EMPTY_MEMORY_REGION: MemoryRegionVm = buildMemoryRegion(null);

export function buildScenarioPrettyVm(payload: ScenarioPayload | null): ScenarioPrettyVm {
	const fog = payload?.fog ?? null;
	const mr = payload?.memory_regions ?? null;

	return {
		hasPayload: !!payload,
		topLevel: {
			map: fieldStr(payload?.map),
			game_difficulty: fieldNum(payload?.game_difficulty, (n) => String(n))
		},
		fog: {
			present: !!fog,
			color: formatColor(fog?.color),
			max_density: fieldNum(fog?.max_density, (n) => n.toFixed(4)),
			atmo_min_dist: fieldNum(fog?.atmo_min_dist, (n) => n.toFixed(2)),
			atmo_max_dist: fieldNum(fog?.atmo_max_dist, (n) => n.toFixed(2))
		},
		memoryRegions: {
			present: !!mr,
			game_state: mr ? buildMemoryRegion(mr.game_state) : EMPTY_MEMORY_REGION,
			tag_cache: mr ? buildMemoryRegion(mr.tag_cache) : EMPTY_MEMORY_REGION,
			texture_cache: mr ? buildMemoryRegion(mr.texture_cache) : EMPTY_MEMORY_REGION,
			sound_cache: mr ? buildMemoryRegion(mr.sound_cache) : EMPTY_MEMORY_REGION
		},
		objectTypes: payload?.object_types ?? [],
		playerSpawns: payload?.player_spawns ?? [],
		powerItemSpawns: payload?.power_item_spawns ?? [],
		tagDefs: sortedTagDefs(payload?.tag_defs)
	};
}

/** Tag-def value formatter — narrows the discriminated union so the Pretty
 * view can render only the fields that exist for that kind. The Pretty
 * component iterates the returned entries directly. */
export interface TagDefField {
	label: string;
	value: string;
	present: boolean;
}

export function tagDefFields(def: ScenarioTagDef): TagDefField[] {
	const out: TagDefField[] = [{ label: 'kind', value: def.kind, present: true }];
	if (def.kind === 'weapon') {
		pushNum(out, 'zoom_levels', def.zoom_levels, (n) => String(n));
		pushNum(out, 'zoom_min', def.zoom_min, (n) => n.toFixed(3));
		pushNum(out, 'zoom_max', def.zoom_max, (n) => n.toFixed(3));
		pushNum(out, 'autoaim_angle', def.autoaim_angle, (n) => n.toFixed(4));
		pushNum(out, 'autoaim_range', def.autoaim_range, (n) => n.toFixed(2));
		pushNum(out, 'magnetism_angle', def.magnetism_angle, (n) => n.toFixed(4));
		pushNum(out, 'magnetism_range', def.magnetism_range, (n) => n.toFixed(2));
		pushNum(out, 'deviation_angle', def.deviation_angle, (n) => n.toFixed(4));
	} else if (def.kind === 'biped') {
		pushNum(out, 'tag_index', def.tag_index, (n) => `${hex8(n)} (${n >>> 0})`);
		pushNum(out, 'flags', def.flags, (n) => `${hex8(n)} (${n >>> 0})`);
		pushNum(out, 'autoaim_pill_radius', def.autoaim_pill_radius, (n) => n.toFixed(3));
	}
	return out;
}

function pushNum(
	out: TagDefField[],
	label: string,
	value: number | undefined,
	render: (n: number) => string
): void {
	if (value === undefined || value === null || !Number.isFinite(value)) {
		out.push({ label, value: '—', present: false });
		return;
	}
	out.push({ label, value: render(value), present: true });
}
