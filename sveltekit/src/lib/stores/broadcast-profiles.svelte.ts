// Resolves each live player's gamertag → their profile appearance (CE armor
// colour + H2 emblem/appearance) so the broadcast cards can render the player's
// PROFILE avatar instead of a generic Spartan.
//
// Live path: batch-GET the public, read-only /api/public/profiles endpoint (the
// owner-scoped ce_profiles/h2_profiles can't be read by an anonymous overlay, so
// this cosmetic-only endpoint exists for exactly this). Results are cached by
// lowercased gamertag and gamertags are only ever fetched once (a gamertag that
// resolves to no profile is remembered as "asked", so the card falls back to a
// generic Spartan and we never re-ask).
//
// Mock path: synthesised from overlay-mock so ?mock=1 previews the feature with
// no backend (one seed intentionally has no profile, to show the fallback).

import { apiBaseURL } from '$lib/utils/api-base';
import { mockProfiles } from '$lib/utils/overlay-mock';
import type { ResolvedProfile } from '$lib/components/broadcast/player';

function key(gamertag: string): string {
	return gamertag.trim().toLowerCase();
}

export function createProfileLookup() {
	// Resolved profiles by lowercased gamertag. $state so cards re-derive when a
	// fetch lands.
	let profiles = $state<Record<string, ResolvedProfile>>({});
	// Lowercased gamertags we've already requested (resolved or not) — never re-ask.
	// A plain object-as-set (not a reactive Set): membership isn't UI state.
	const asked: Record<string, true> = {};
	let mockLoaded = false;

	/** Ensure profiles for these gamertags are loaded. Mock: fill from the mock
	 * table once. Live: fetch the not-yet-asked ones in one batch (deduped as we go). */
	function ensure(gamertags: string[], mock: boolean): void {
		if (mock) {
			if (mockLoaded) return;
			mockLoaded = true;
			profiles = { ...profiles, ...mockProfiles() };
			return;
		}
		const toFetch: string[] = [];
		for (const g of gamertags) {
			const k = key(g);
			if (k.length === 0 || asked[k]) continue;
			asked[k] = true;
			toFetch.push(k);
		}
		if (toFetch.length === 0) return;
		void fetchBatch(toFetch);
	}

	async function fetchBatch(tags: string[]): Promise<void> {
		try {
			const base = apiBaseURL();
			const url = `${base}/api/public/profiles?gamertags=${encodeURIComponent(tags.join(','))}`;
			const res = await fetch(url);
			if (!res.ok) return;
			const body = (await res.json()) as { profiles?: Record<string, ResolvedProfile> };
			if (!body.profiles) return;
			// The endpoint returns the PB avatar as a same-origin relative path
			// (/api/files/...). Resolve it against the PB origin so the <img>
			// works in dev, where the overlay page runs on Vite's port.
			const merged: Record<string, ResolvedProfile> = {};
			for (const [k, p] of Object.entries(body.profiles)) {
				merged[k] = p.avatar && p.avatar.startsWith('/') ? { ...p, avatar: base + p.avatar } : p;
			}
			profiles = { ...profiles, ...merged };
		} catch {
			// Network/parse failure → cards fall back to generic Spartans. The tags
			// stay in `asked`, so a flaky endpoint isn't hammered every tick.
		}
	}

	return {
		ensure,
		/** The resolved profile for a live player name, or null (→ generic Spartan). */
		get(gamertag: string): ResolvedProfile | null {
			return profiles[key(gamertag)] ?? null;
		}
	};
}

export type ProfileLookup = ReturnType<typeof createProfileLookup>;
