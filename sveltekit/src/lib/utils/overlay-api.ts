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

/** One live scraper instance for the Studio picker: the container `name` (the
 *  id overlays target) + the friendly `xbox_name` (the console name). */
export interface OverlayInstance {
	name: string; // container / instance id — what the overlay ?instance= wants
	xbox_name: string; // friendly console name (may be empty)
}

/** One live console for the Studio picker — its name plus which host currently
 *  sees it. `machine_index` -1 = the host's own console (no live lobby). */
export interface OverlayConsole {
	console: string;
	instance: string;
	is_local: boolean;
	machine_index: number;
}

/** List every console name currently visible across all hosts (each host's own
 *  console + its System Link lobby peers) — the console index the overlays
 *  resolve against. Public PoC endpoint; overlays target by these names alone
 *  (no instance / token). Returns [] on error. */
export async function listConsoles(): Promise<OverlayConsole[]> {
	try {
		const res = await fetch(`${apiBaseURL()}/api/overlay/consoles`);
		if (!res.ok) return [];
		const data = await res.json();
		return (data?.consoles ?? []) as OverlayConsole[];
	} catch {
		return [];
	}
}

/** List the live scraper instances so Studio can offer a pick-by-friendly-name
 *  dropdown (targeting by container id under the hood). Reads the admin scraper
 *  endpoint; returns [] if the caller isn't an admin (a non-admin overlay
 *  manager falls back to typing the instance id manually). */
export async function listOverlayInstances(): Promise<OverlayInstance[]> {
	try {
		const res = await fetch(`${apiBaseURL()}/api/admin/scraper`, {
			headers: { Authorization: auth.token }
		});
		if (!res.ok) return [];
		const rows = (await res.json()) as Array<{ name?: string; xbox_name?: string }>;
		return (rows ?? [])
			.filter((r) => !!r.name)
			.map((r) => ({ name: r.name as string, xbox_name: r.xbox_name ?? '' }));
	} catch {
		return [];
	}
}
