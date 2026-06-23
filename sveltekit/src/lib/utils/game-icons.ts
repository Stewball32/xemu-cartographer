// Tag → real-game-icon lookup for the top-down visualizer minimap.
//
// The actual sprite PNGs are decoded from the user's own Halo game files by
// scripts/game-icons/extract_icons.py into a git-ignored, visualizer-served
// cache (sveltekit/static/game-icons/<game>/) with a manifest.json. This module
// is the consumer: PURE tag→icon-key resolution (unit-tested, no IO) plus a
// thin, failure-tolerant manifest loader.
//
// Resolution is two-tier:
//   1. the manifest's curated `tag_map` (authoritative weapon keys + the
//      powerup → waypoint-marker mapping that CAN'T be derived from the tag,
//      since stock CE has no powerup HUD sprite and the modded build marks
//      powerups with repurposed waypoint glyphs), then
//   2. an OPTIMISTIC derived key (weapon_<folder>, vehicle_<folder>, grenade …)
//      gated on what the manifest actually ships.
// So CE lights up weapons / grenade / objective + powerup-as-waypoint and
// leaves vehicles on the generic fallback, while an H2 manifest that adds
// vehicle_* / its own tag_map entries lights them up with no frontend change.
// If the cache was never regenerated, loadIconSet returns an empty set and
// every marker cleanly falls back to its generic shape.

import { base } from '$app/paths';
import type { TagClass } from '$lib/utils/visualizer-view';

/** Slugify a tag folder name to a key segment — MUST match the extractor's
 * Python slugify (lowercase, non-alphanumeric runs → '_', trimmed). */
export function slugifyTagSegment(s: string): string {
	return s
		.trim()
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '');
}

/** The folder name that identifies an object's type — the second segment of a
 * Halo tag path ("weapons\\assault rifle\\assault rifle" → "assault rifle"),
 * falling back to the last segment for 2-part paths. */
function tagFolder(tag: string): string {
	const parts = (tag ?? '').split(/[\\/]/).filter(Boolean);
	if (parts.length >= 2) return parts[1];
	return parts[parts.length - 1] ?? '';
}

/** Curated tag→key tables shipped in the manifest (one per category). */
export interface TagMap {
	weapon: Record<string, string>;
	powerup: Record<string, string>;
	equipment: Record<string, string>;
	grenade: Record<string, string>; // { "*": key }
	objective: Record<string, string>;
	vehicle: Record<string, string>;
}

const EMPTY_TAG_MAP: TagMap = {
	weapon: {},
	powerup: {},
	equipment: {},
	grenade: {},
	objective: {},
	vehicle: {}
};

function normalizeTagMap(raw: Partial<TagMap> | undefined): TagMap {
	return { ...EMPTY_TAG_MAP, ...(raw ?? {}) };
}

/**
 * DERIVED icon key for a world item from its tag + classified kind (the
 * optimistic fallback used when the manifest's tag_map has no curated entry).
 * Grenades are detected by substring first (CE files them under weapons\\, so
 * classifyTag calls them 'weapon'); a single generic grenade glyph covers
 * frag + plasma (verified shared in stock CE).
 */
export function iconKeyForItem(tag: string | null | undefined, kind: TagClass): string | null {
	const t = (tag ?? '').toLowerCase();
	if (t.includes('grenade')) return 'grenade';
	const folder = tagFolder(tag ?? '');
	if (!folder) return null;
	switch (kind) {
		case 'weapon':
			return `weapon_${slugifyTagSegment(folder)}`;
		case 'equipment':
			return `equipment_${slugifyTagSegment(folder)}`;
		// powerups have no derivable key — they only resolve via the curated
		// tag_map (waypoint markers), so derived returns null here.
		default:
			return null;
	}
}

/** DERIVED vehicle icon key (CE ships none → gated out by the manifest; H2 can
 * add vehicle_<folder> sprites and they light up with no other change). */
export function iconKeyForVehicle(tag: string | null | undefined): string | null {
	const folder = tagFolder(tag ?? '');
	return folder ? `vehicle_${slugifyTagSegment(folder)}` : null;
}

/** Objective-marker key for a CTF flag. */
export const OBJECTIVE_FLAG_KEY = 'objective_flag';

/** Resolve a world item's icon KEY: curated tag_map first (authoritative weapon
 * keys + powerup→waypoint-marker mapping), then the derived fallback. */
export function resolveItemKey(
	tagMap: TagMap,
	tag: string | null | undefined,
	kind: TagClass
): string | null {
	const t = (tag ?? '').toLowerCase();
	if (t.includes('grenade')) return tagMap.grenade['*'] ?? 'grenade';
	const slug = slugifyTagSegment(tagFolder(tag ?? ''));
	if (slug) {
		const table =
			kind === 'weapon'
				? tagMap.weapon
				: kind === 'powerup'
					? tagMap.powerup
					: kind === 'equipment'
						? tagMap.equipment
						: null;
		const curated = table?.[slug];
		if (curated) return curated;
	}
	return iconKeyForItem(tag, kind);
}

/** Resolve a vehicle's icon KEY: curated tag_map first, then derived. */
export function resolveVehicleKey(tagMap: TagMap, tag: string | null | undefined): string | null {
	const slug = slugifyTagSegment(tagFolder(tag ?? ''));
	return (slug && tagMap.vehicle[slug]) || iconKeyForVehicle(tag);
}

/** Resolve the CTF-flag marker KEY (tag_map override, else the default). */
export function resolveFlagKey(tagMap: TagMap): string {
	return tagMap.objective.flag ?? OBJECTIVE_FLAG_KEY;
}

// ---------------------------------------------------------------------------

interface IconManifest {
	game: string;
	icons: Record<string, { file: string }>;
	tag_map?: Partial<TagMap>;
}

/** A resolved, in-memory icon set: which keys exist + their served URLs, plus
 * tag-aware resolvers for the visualizer's item / vehicle / flag markers. */
export interface IconSet {
	game: string;
	/** True when at least one icon loaded (cache was regenerated). */
	loaded: boolean;
	/** URL for an explicit key, or null when this game/cache doesn't ship it. */
	url(key: string | null | undefined): string | null;
	/** URL for a world item (weapon / grenade / powerup), or null → fallback. */
	itemUrl(tag: string | null | undefined, kind: TagClass): string | null;
	/** URL for a vehicle, or null → fallback rectangle. */
	vehicleUrl(tag: string | null | undefined): string | null;
	/** URL for a CTF flag, or null → fallback flag glyph. */
	flagUrl(): string | null;
}

function makeIconSet(game: string, urls: Map<string, string>, tagMap: TagMap): IconSet {
	const url = (key: string | null | undefined) => (key ? (urls.get(key) ?? null) : null);
	return {
		game,
		loaded: urls.size > 0,
		url,
		itemUrl: (tag, kind) => url(resolveItemKey(tagMap, tag, kind)),
		vehicleUrl: (tag) => url(resolveVehicleKey(tagMap, tag)),
		flagUrl: () => url(resolveFlagKey(tagMap))
	};
}

/** An always-empty set — every marker falls back to its generic shape. */
export function emptyIconSet(game: string): IconSet {
	return makeIconSet(game, new Map(), EMPTY_TAG_MAP);
}

function buildIconSet(game: string, man: IconManifest): IconSet {
	const urls = new Map<string, string>();
	for (const [key, meta] of Object.entries(man.icons ?? {})) {
		if (meta?.file) urls.set(key, `${base}/game-icons/${game}/${meta.file}`);
	}
	return makeIconSet(game, urls, normalizeTagMap(man.tag_map));
}

/**
 * Load the per-game icon manifest. Resilient by design: any failure (cache
 * never regenerated → 404, malformed JSON, offline) resolves to an empty set
 * so the visualizer renders with generic markers rather than throwing.
 */
export async function loadIconSet(game: string, fetchFn: typeof fetch = fetch): Promise<IconSet> {
	try {
		const res = await fetchFn(`${base}/game-icons/${game}/manifest.json`);
		if (!res.ok) return emptyIconSet(game);
		const man = (await res.json()) as IconManifest;
		return buildIconSet(game, man);
	} catch {
		return emptyIconSet(game);
	}
}
