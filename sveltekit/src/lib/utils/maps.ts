// Map list + thumbnail helpers, shared by the organizer game-management page and
// the play catalog. A build's maps are extracted at ingest into iso_maps rows;
// multiplayer maps carry a top-down BSP-render thumbnail (CE has no embedded
// preview art, so the render IS the map graphic).
import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

export interface IsoMap {
	id: string;
	filename: string;
	name: string;
	map_type: string; // campaign / multiplayer / ui
	/** relative file path, or "" until the thumbnail renders. */
	thumb_url: string;
	thumb_status: string; // pending / ready / failed / skipped
}

/** Absolute thumbnail URL for a map row, or null when none is available. */
export function mapThumbURL(m: IsoMap): string | null {
	return m.thumb_url ? `${apiBaseURL()}${m.thumb_url}` : null;
}

function authHeaders(): Record<string, string> {
	const h: Record<string, string> = {};
	if (auth.token) h.Authorization = auth.token;
	return h;
}

/** GET the maps for a build. `scope` picks the endpoint: the organizer/admin
 * catalog route or the player-scoped play route (same shape, different gate). */
export async function listIsoMaps(
	id: string,
	scope: 'admin' | 'play' = 'admin'
): Promise<IsoMap[]> {
	const base = scope === 'play' ? '/api/play/isos' : '/api/admin/isos';
	const res = await fetch(`${apiBaseURL()}${base}/${encodeURIComponent(id)}/maps`, {
		headers: authHeaders()
	});
	if (!res.ok) return [];
	return (await res.json()) as IsoMap[];
}
