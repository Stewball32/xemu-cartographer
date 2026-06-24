// Client for the LAN-saves endpoints (/api/lan/saves/*).
//
// These are NOT under /api/admin, so they don't use the admin-api helper. They
// are open on a trusted LAN by default (or gated by a shared LAN token); we
// still attach the PocketBase JWT when present so an admin operator is
// recognised regardless of the token config.
import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';
import type { BuildRequest, BuildResponse, DownloadFormat, LanMeta } from '$lib/types/lansaves';

export class LanSavesError extends Error {
	status: number;
	body: unknown;
	constructor(status: number, message: string, body?: unknown) {
		super(message);
		this.status = status;
		this.body = body;
		this.name = 'LanSavesError';
	}
}

function authHeaders(json = false): Record<string, string> {
	const h: Record<string, string> = {};
	if (auth.token) h.Authorization = auth.token;
	if (json) h['Content-Type'] = 'application/json';
	return h;
}

async function errorFrom(res: Response): Promise<LanSavesError> {
	let msg = `HTTP ${res.status}`;
	let body: unknown;
	try {
		body = await res.clone().json();
		if (body && typeof body === 'object' && 'error' in body && typeof body.error === 'string') {
			msg = (body as { error: string }).error;
		}
	} catch {
		/* non-JSON */
	}
	return new LanSavesError(res.status, msg, body);
}

/** Strip undefined/empty fields so the request only carries what it sets. */
function clean(req: BuildRequest): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const [k, v] of Object.entries(req)) {
		if (v === undefined || v === null || v === '') continue;
		if (k === 'appearance') {
			const ap = v as Record<string, number>;
			const filtered = Object.fromEntries(Object.entries(ap).filter(([, n]) => Number.isFinite(n)));
			if (Object.keys(filtered).length) out.appearance = filtered;
			continue;
		}
		out[k] = v;
	}
	return out;
}

/** GET /api/lan/saves/meta — generator capabilities for the editor UI. */
export async function lanMeta(): Promise<LanMeta> {
	const res = await fetch(`${apiBaseURL()}/api/lan/saves/meta`, { headers: authHeaders() });
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as LanMeta;
}

/** POST /api/lan/saves/build — preview + round-trip validate (no file bytes). */
export async function lanBuild(req: BuildRequest): Promise<BuildResponse> {
	const res = await fetch(`${apiBaseURL()}/api/lan/saves/build`, {
		method: 'POST',
		headers: authHeaders(true),
		body: JSON.stringify(clean(req))
	});
	if (!res.ok) throw await errorFrom(res);
	return (await res.json()) as BuildResponse;
}

/** Build the GET URL the nxdk LAN client hits to download the save. */
export function lanDownloadURL(
	req: BuildRequest,
	format: DownloadFormat = 'tar',
	freeBytes?: number
): string {
	const q = new URLSearchParams();
	for (const [k, v] of Object.entries(clean(req))) {
		if (k === 'appearance') {
			for (const [ak, av] of Object.entries(v as Record<string, number>)) {
				q.set(`app_${ak}`, String(av));
			}
			continue;
		}
		q.set(k, String(v));
	}
	q.set('format', format);
	if (freeBytes !== undefined && Number.isFinite(freeBytes)) q.set('free_bytes', String(freeBytes));
	return `${apiBaseURL()}/api/lan/saves/download?${q.toString()}`;
}

/**
 * POST /api/lan/saves/download and trigger a browser save of the resulting
 * file. Returns the suggested filename. Throws LanSavesError (status 507) when
 * the disk-space check fails.
 */
export async function lanDownload(
	req: BuildRequest,
	format: DownloadFormat = 'tar',
	freeBytes?: number
): Promise<string> {
	const body: Record<string, unknown> = { ...clean(req), format };
	if (freeBytes !== undefined && Number.isFinite(freeBytes)) body.free_bytes = freeBytes;
	const res = await fetch(`${apiBaseURL()}/api/lan/saves/download`, {
		method: 'POST',
		headers: authHeaders(true),
		body: JSON.stringify(body)
	});
	if (!res.ok) throw await errorFrom(res);

	const blob = await res.blob();
	const filename =
		filenameFromDisposition(res.headers.get('Content-Disposition')) ?? `${req.name}.${format}`;
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	a.remove();
	URL.revokeObjectURL(url);
	return filename;
}

function filenameFromDisposition(value: string | null): string | undefined {
	if (!value) return undefined;
	const m = /filename="?([^"]+)"?/.exec(value);
	return m?.[1];
}
