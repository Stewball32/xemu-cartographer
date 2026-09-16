// Client for the overlay console index (/api/overlay/consoles) + the Studio
// access gate. authz (design §9): the console index is gated by the
// `overlay.list_consoles` scope server-side, Studio by `overlay.mint` — the
// scope that lets the caller mint spectator keys (`tokens-api.ts`). Overlays
// themselves still reach the WS through the anonymous `?console=` door, or
// through a spectator key (`?spectator=`) minted from Studio.

import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

/** True when the current user may use Studio: holds `overlay.mint` (the seed
 *  admin / overlay_manager roles carry it; superusers hold "*"). */
export function canManageOverlays(): boolean {
	return auth.hasScope('overlay.mint');
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
 *  resolve against. Sends the caller's PB JWT; the server answers 401 to
 *  anonymous callers and 403 without `overlay.list_consoles`, both of which
 *  (like any other failure) collapse to []. */
export async function listConsoles(): Promise<OverlayConsole[]> {
	try {
		const headers: Record<string, string> = {};
		if (auth.token) headers.Authorization = auth.token;
		const res = await fetch(`${apiBaseURL()}/api/overlay/consoles`, { headers });
		if (res.status === 401 || res.status === 403) return [];
		if (!res.ok) return [];
		const data = await res.json();
		return (data?.consoles ?? []) as OverlayConsole[];
	} catch {
		return [];
	}
}
