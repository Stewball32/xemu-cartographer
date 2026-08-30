// Resolves each live player's scraped in-game name → their broadcast identity
// (default gamertag + avatar) so the overlays can show who is actually playing
// instead of whatever the console happened to be logged in as.
//
// Restored from the M28 broadcast-cards store deleted in 58459f2 — the caching
// shape was already right, only the response type and naming changed.
//
// Live path: batch-GET the public, read-only /api/public/profiles endpoint. The
// underlying users / gamertags / ce_profiles / h2_profiles collections are all
// owner-or-admin scoped and cannot be read by an anonymous overlay, so that
// endpoint exists for exactly this. It resolves through the MODERATED gamertags
// collection, so an unreviewed name never reaches the stream.
//
// Results are cached by lowercased scraped name, and each name is only ever
// asked once — a name that resolves to nothing is remembered as "asked", so an
// unidentified player falls back to their trimmed in-game name + the placeholder
// emblem and we never re-ask. That matters: the roster is re-derived on every
// tick (~30Hz), and without the negative cache this would hammer the endpoint.
//
// Mock path: synthesised from overlay-mock so ?mock=1 previews the feature with
// no backend (one seed intentionally has no profile, to show the fallback).

import { apiBaseURL } from '$lib/utils/api-base';
import { mockProfiles } from '$lib/utils/overlay-mock';
import type { OverlayIdentity } from '$lib/utils/overlay-state';

function key(name: string): string {
	return name.trim().toLowerCase();
}

export function createProfileLookup() {
	// Resolved identities by lowercased scraped name. $state so the overlays
	// re-derive when a fetch lands.
	let profiles = $state<Record<string, OverlayIdentity>>({});
	// Names we've already requested (resolved or not) — never re-ask. A plain
	// object-as-set, not a reactive Set: membership isn't UI state.
	const asked: Record<string, true> = {};
	let mockLoaded = false;

	/** Ensure identities for these scraped names are loaded. Mock: fill from the
	 * mock table once. Live: fetch the not-yet-asked ones in one batch. */
	function ensure(names: string[], mock: boolean): void {
		if (mock) {
			if (mockLoaded) return;
			mockLoaded = true;
			profiles = { ...profiles, ...mockProfiles() };
			return;
		}
		const toFetch: string[] = [];
		for (const n of names) {
			const k = key(n);
			if (k.length === 0 || asked[k]) continue;
			asked[k] = true;
			toFetch.push(k);
		}
		if (toFetch.length === 0) return;
		void fetchBatch(toFetch);
	}

	async function fetchBatch(names: string[]): Promise<void> {
		try {
			const base = apiBaseURL();
			const url = `${base}/api/public/profiles?gamertags=${encodeURIComponent(names.join(','))}`;
			const res = await fetch(url);
			if (!res.ok) return;
			const body = (await res.json()) as { profiles?: Record<string, OverlayIdentity> };
			if (!body.profiles) return;
			// The endpoint returns file paths (avatar, plate art) as same-origin
			// relative paths (/api/files/...). Resolve them against the PB origin
			// so the assets load in dev, where the overlay page runs on Vite's
			// port.
			const merged: Record<string, OverlayIdentity> = {};
			for (const [k, p] of Object.entries(body.profiles)) {
				merged[k] = {
					...p,
					avatar: p.avatar?.startsWith('/') ? base + p.avatar : p.avatar,
					plate: p.plate?.startsWith('/') ? base + p.plate : p.plate
				};
			}
			profiles = { ...profiles, ...merged };
		} catch {
			// Network/parse failure → players keep their scraped names and the
			// placeholder emblem. The names stay in `asked`, so a flaky endpoint
			// isn't hammered every tick.
		}
	}

	return {
		ensure,
		/** All resolved identities, keyed by lowercased scraped name — hand
		 * straight to applyIdentities(). */
		get all(): Record<string, OverlayIdentity> {
			return profiles;
		}
	};
}

export type ProfileLookup = ReturnType<typeof createProfileLookup>;
