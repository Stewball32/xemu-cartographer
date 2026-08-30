// Client for GET /api/me — the caller's identity (gamertags + default), shared
// by the settings tabs. (Team membership is also on the wire but currently
// PARKED: the redesigned settings page doesn't surface it until the Teams tab
// gets its own design pass.)
import { auth } from '$lib/stores/auth.svelte';
import { apiBaseURL } from '$lib/utils/api-base';

export interface MeGamertag {
	id: string;
	tag: string;
	status: string;
}

export interface MeIdentity {
	default_gamertag: MeGamertag | null;
	gamertags: MeGamertag[];
}

/** Fetch the caller's gamertag identity; null on any failure (the tab just
 * renders empty — a network blip shouldn't toast). */
export async function fetchIdentity(): Promise<MeIdentity | null> {
	if (!auth.token) return null;
	try {
		const res = await fetch(`${apiBaseURL()}/api/me`, {
			headers: { Authorization: auth.token }
		});
		if (!res.ok) return null;
		const data = (await res.json()) as MeIdentity;
		return {
			default_gamertag: data.default_gamertag ?? null,
			gamertags: data.gamertags ?? []
		};
	} catch {
		return null;
	}
}
