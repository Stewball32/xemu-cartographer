// Tag → real-game-icon lookup for the top-down visualizer minimap.
//
// The actual sprite PNGs are decoded from the user's own Halo game files by
// scripts/game-icons/extract_icons.py into a git-ignored, visualizer-served
// cache (sveltekit/static/game-icons/<game>/) with a manifest.json. This module
// is the consumer: a PURE tag→icon-key derivation (unit-tested, no IO) plus a
// thin, failure-tolerant manifest loader.
//
// Design: keys are derived OPTIMISTICALLY from the object's tag + class
// (weapon_<folder>, powerup_<folder>, vehicle_<folder>, grenade,
// objective_flag, …) and then gated on what the loaded manifest actually
// ships. So CE — which only has weapon / grenade / objective sprites — lights
// those up and leaves powerups / vehicles on the generic fallback marker, while
// an H2 manifest that adds powerup_* / vehicle_* sprites lights them up with no
// frontend change. If the cache was never regenerated, loadIconSet returns an
// empty set and every marker cleanly falls back.

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

/**
 * Derive the icon key for a world item / object from its tag + classified kind,
 * or null when the kind carries no per-type icon. Grenades are detected by
 * substring first (CE files them under weapons\\, so classifyTag calls them
 * 'weapon'); a single generic grenade glyph covers frag + plasma.
 */
export function iconKeyForItem(tag: string | null | undefined, kind: TagClass): string | null {
	const t = (tag ?? '').toLowerCase();
	if (t.includes('grenade')) return 'grenade';
	const folder = tagFolder(tag ?? '');
	if (!folder) return null;
	switch (kind) {
		case 'weapon':
			return `weapon_${slugifyTagSegment(folder)}`;
		case 'powerup':
			return `powerup_${slugifyTagSegment(folder)}`;
		case 'equipment':
			return `equipment_${slugifyTagSegment(folder)}`;
		default:
			return null;
	}
}

/** Vehicle icon key (CE ships none → gated out by the manifest; H2 can add
 * vehicle_<folder> sprites and they light up with no other change). */
export function iconKeyForVehicle(tag: string | null | undefined): string | null {
	const folder = tagFolder(tag ?? '');
	return folder ? `vehicle_${slugifyTagSegment(folder)}` : null;
}

/** Objective-marker key for a CTF flag. Oddball / KotH keys
 * (objective_oddball / objective_koth) exist in the manifest for when the feed
 * surfaces those objects. */
export const OBJECTIVE_FLAG_KEY = 'objective_flag';

// ---------------------------------------------------------------------------

interface IconManifest {
	game: string;
	icons: Record<string, { file: string }>;
}

/** A resolved, in-memory icon set: which keys exist + their served URLs. */
export interface IconSet {
	game: string;
	/** True when at least one icon loaded (cache was regenerated). */
	loaded: boolean;
	/** URL for a derived key, or null when this game/cache doesn't ship it. */
	url(key: string | null | undefined): string | null;
}

/** An always-empty set — every marker falls back to its generic shape. */
export function emptyIconSet(game: string): IconSet {
	return { game, loaded: false, url: () => null };
}

function buildIconSet(game: string, man: IconManifest): IconSet {
	const urls = new Map<string, string>();
	for (const [key, meta] of Object.entries(man.icons ?? {})) {
		if (meta?.file) urls.set(key, `${base}/game-icons/${game}/${meta.file}`);
	}
	return {
		game,
		loaded: urls.size > 0,
		url: (key) => (key ? (urls.get(key) ?? null) : null)
	};
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
