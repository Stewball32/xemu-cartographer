// Client for the admin ISO-catalog endpoints (/api/admin/isos/*).
//
// Ingest model: discs are dropped into the tier-root inbox (inbox/isos/) and
// ingested into the managed library, where each row's disc lives as <id>.iso —
// hashed (content_hash, the drift anchor), frozen read-only, then extracted. The
// display `name` is decoupled from the file and freely editable. A row whose
// managed bytes stop matching its hash is flagged drift_detected + forced
// unavailable. server_iso optionally links another catalog entry as the game's
// server build.
//
// All routes are RequireAuth + RequireAdmin; we attach the PocketBase JWT.
import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

/** A catalog entry as projected by the admin isos routes (isoView). */
export interface IsoEntry {
	id: string;
	name: string;
	/** original inbox filename — provenance only (managed file is <id>.iso). */
	filename: string;
	title_id: string;
	description: string;
	available: boolean;
	server_iso: string;
	/** offset-set id the scraper binds for this build ("" = game baseline). */
	offset_set: string;
	/** sha256 of the managed disc (drift anchor). */
	content_hash: string;
	/** managed bytes no longer match content_hash → forced unavailable. */
	drift_detected: boolean;
	file_size: number;
	extracted_ready: boolean;
	extracted_at: string;
	footprint_bytes: number;
	created: string;
	updated: string;
}

/** A disc image staged in the inbox, pending ingest. */
export interface InboxFile {
	filename: string;
	size: number;
}

export interface IngestedItem {
	id: string;
	name: string;
	filename: string;
	hash: string;
	immutable: boolean;
}
export interface SkippedItem {
	filename: string;
	reason: string;
	dup_of?: string;
}
export interface IngestResult {
	ingested: IngestedItem[];
	skipped: SkippedItem[];
	errors: string[];
}

/** PATCH body — every field optional; server_iso "" clears the link. */
export interface IsoUpdate {
	name?: string;
	title_id?: string;
	description?: string;
	available?: boolean;
	server_iso?: string;
	offset_set?: string;
}

/** One registered memory-offset set (version-level address layer). */
export interface OffsetSetInfo {
	game: string;
	id: string;
	description: string;
	count: number;
	baseline: boolean;
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

/** GET /api/admin/isos/inbox — disc images staged for ingest. */
export async function scanInbox(): Promise<InboxFile[]> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/inbox`, { headers: authHeaders() });
	if (!res.ok) throw await errorFrom(res);
	const body = (await res.json()) as { files: InboxFile[] };
	return body.files ?? [];
}

/** POST /api/admin/isos/ingest — scan the inbox + ingest every pending file. */
export async function ingestInbox(): Promise<IngestResult> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/ingest`, {
		method: 'POST',
		headers: authHeaders()
	});
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as IngestResult;
}

/** GET /api/admin/isos/offset-sets — registered offset sets for the picker. */
export async function listOffsetSets(): Promise<OffsetSetInfo[]> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/offset-sets`, { headers: authHeaders() });
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as OffsetSetInfo[];
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

/** DELETE /api/admin/isos/{id} — remove the entry AND its managed disc + tree. */
export async function deleteIso(id: string): Promise<void> {
	const res = await fetch(`${apiBaseURL()}/api/admin/isos/${encodeURIComponent(id)}`, {
		method: 'DELETE',
		headers: authHeaders()
	});
	if (!res.ok) throw await errorFrom(res);
}

/** Human-readable size for a byte count (0 → "—"). */
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

/** First 12 chars of a hash for compact display ("—" when empty). */
export function shortHash(h: string): string {
	return h ? h.slice(0, 12) : '—';
}
