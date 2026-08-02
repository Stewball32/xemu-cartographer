// Client for the admin ISO-catalog endpoints (/api/admin/isos/*).
//
// The catalog is a set of records over the on-disk ISO library dir on the host
// (podman Config.ISODir). Each row names a bare filename plus player-facing
// metadata and, for a game, an optional server_iso link to another catalog
// entry (its dedicated server build). The library file a row points at is
// WRITE-ONCE (isos_immutable): metadata + server_iso stay editable, but to
// point at a different disc you delete the row and register the new file.
//
// All routes are RequireAuth + RequireAdmin; we attach the PocketBase JWT.
import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

/** A catalog entry as projected by the admin isos routes (isoView). */
export interface IsoEntry {
	id: string;
	name: string;
	filename: string;
	title_id: string;
	description: string;
	available: boolean;
	/** id of another catalog entry that is this game's server build ("" = none). */
	server_iso: string;
	/** derived extract-cache status (isos_extract hook). */
	extracted_ready: boolean;
	extracted_at: string;
	footprint_bytes: number;
	created: string;
	updated: string;
}

/** One disc image present in the shared ISO library dir. */
export interface LibraryFile {
	filename: string;
	registered: boolean;
	iso_id: string;
}

export interface Library {
	dir: string;
	files: LibraryFile[];
}

export interface IsoCreate {
	name: string;
	filename: string;
	title_id?: string;
	description?: string;
	available?: boolean;
	server_iso?: string;
}

/** PATCH body — every field optional; server_iso "" clears the link. */
export interface IsoUpdate {
	name?: string;
	title_id?: string;
	description?: string;
	available?: boolean;
	server_iso?: string;
}

export class IsoApiError extends Error {
	status: number;
	constructor(status: number, message: string) {
		super(message);
		this.status = status;
		this.name = 'IsoApiError';
	}
}

function authHeaders(json = false): Record<string, string> {
	const h: Record<string, string> = {};
	if (auth.token) h.Authorization = auth.token;
	if (json) h['Content-Type'] = 'application/json';
	return h;
}

async function errorFrom(res: Response): Promise<IsoApiError> {
	let msg = `HTTP ${res.status}`;
	try {
		const body = await res.clone().json();
		if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
			msg = body.error;
		}
	} catch {
		/* non-JSON */
	}
	return new IsoApiError(res.status, msg);
}

/** GET /api/admin/isos — every catalog entry. */
export async function listIsos(): Promise<IsoEntry[]> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos`, { headers: authHeaders() });
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as IsoEntry[];
}

/**
 * GET /api/admin/isos/library — disc images on disk, flagged registered-or-not.
 * 503 when the container subsystem is disabled (no host library to scan).
 */
export async function scanLibrary(): Promise<Library> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/library`, { headers: authHeaders() });
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as Library;
}

/** POST /api/admin/isos — register a library file as a catalog entry. */
export async function createIso(body: IsoCreate): Promise<IsoEntry> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos`, {
		method: 'POST',
		headers: authHeaders(true),
		body: JSON.stringify(body)
	});
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as IsoEntry;
}

/** PATCH /api/admin/isos/{id} — partial metadata/server_iso/available update. */
export async function updateIso(id: string, body: IsoUpdate): Promise<IsoEntry> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		headers: authHeaders(true),
		body: JSON.stringify(body)
	});
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as IsoEntry;
}

/** DELETE /api/admin/isos/{id} — remove the catalog entry (disk file untouched). */
export async function deleteIso(id: string): Promise<void> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: authHeaders()
	});
	if (!res.ok) throw await errorFrom(res);
}

/** Human-readable size for footprint_bytes (0 → "—"). */
export function formatBytes(n: number): string {
	if (!n || n <= 0) return '—';
	const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
	let v = n;
	let u = 0;
	while (v >= 1024 && u < units.length - 1) {
		v /= 1024;
		u++;
	}
	return `${v.toFixed(u === 0 ? 0 : 1)} ${units[u]}`;
}
