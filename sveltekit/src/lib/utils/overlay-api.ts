// Client for the overlay console index (/api/overlay/consoles) + the Studio
// access gate. The M10 overlay-token endpoints are gone — overlays target by
// console name over the tokenless WS door (auth deferred; a future auth story
// will replace it wholesale).

import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

/** True when the current user may use Studio (admin/superuser or the
 *  overlay_manager role). */
export function canManageOverlays(): boolean {
	return auth.isAdmin || auth.hasRole('overlay_manager');
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
