// Client for the M10 overlay-token endpoints (/api/overlay-tokens/*). The
// backend gates mint/list/revoke on the "manage overlays" permission
// (admin/superuser or the overlay_manager role); these helpers just carry the
// caller's JWT.

import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

export interface OverlayToken {
	kid: string;
	room: string;
	label: string;
	expires_at: string;
	created: string;
	minted_by: string;
}

export interface MintResult {
	token: string;
	kid: string;
	room: string;
	expires_at: string;
	/** Overlay page URL (render page deferred; path shape stable). */
	url: string;
	/** Raw WS endpoint carrying the token. */
	ws_url: string;
}

async function request(method: string, path: string, body?: unknown): Promise<Response> {
	const headers: Record<string, string> = { Authorization: auth.token };
	if (body !== undefined) headers['Content-Type'] = 'application/json';
	return fetch(`${apiBaseURL()}/api/overlay-tokens${path}`, {
		method,
		headers,
		body: body !== undefined ? JSON.stringify(body) : undefined
	});
}

export async function mintOverlayToken(
	room: string,
	label: string,
	ttlSeconds?: number
): Promise<MintResult> {
	const res = await request('POST', '', { room, label, ttl_seconds: ttlSeconds ?? 0 });
	if (!res.ok) {
		const msg = await res.json().catch(() => null);
		throw new Error(msg?.error ?? `mint failed (${res.status})`);
	}
	return res.json();
}

export async function listOverlayTokens(): Promise<OverlayToken[]> {
	const res = await request('GET', '');
	if (!res.ok) throw new Error(`list failed (${res.status})`);
	const data = await res.json();
	return data?.tokens ?? [];
}

export async function revokeOverlayToken(kid: string): Promise<void> {
	const res = await request('POST', `/${encodeURIComponent(kid)}/revoke`);
	if (!res.ok) throw new Error(`revoke failed (${res.status})`);
}

/** True when the current user may manage overlay tokens (mirrors the backend
 *  canManageOverlays gate: admin/superuser or the overlay_manager role). */
export function canManageOverlays(): boolean {
	return auth.isAdmin || auth.hasRole('overlay_manager');
}
