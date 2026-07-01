// Shared query/param parsing for the broadcast OBS routes. Every broadcast
// browser source takes the same shape off its URL — `[instance]` from the path,
// then `?token=`, `?mock=1`, `?scale=`, and `?game=ce|h2` from the query — so the
// three routes (scoreboard / cards / single card) load identically. Kept pure so
// it unit-tests without a SvelteKit load context.

import { parseGame, type BroadcastGame } from '$lib/components/broadcast/theme';

export interface OverlayParams {
	instance: string;
	/** M10 scoped overlay token (the WS credential); '' when previewing. */
	token: string;
	/** ?mock=1 → animated sample data, no socket/token needed. */
	mock: boolean;
	/** OBS sizing multiplier, clamped 0.5..3. */
	scale: number;
	/** Which game's visual language to render. Defaults to CE. */
	game: BroadcastGame;
}

/** Clamp the `?scale=` multiplier to a sane OBS range (default 1). */
export function parseScale(raw: string | null): number {
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) return 1;
	return Math.min(3, Math.max(0.5, n));
}

/** Project a broadcast route's path param + URL into the shared param bundle. */
export function overlayParams(instance: string, url: URL): OverlayParams {
	const mockParam = url.searchParams.get('mock');
	return {
		instance,
		token: url.searchParams.get('token') ?? '',
		mock: mockParam === '1' || mockParam === 'true',
		scale: parseScale(url.searchParams.get('scale')),
		game: parseGame(url.searchParams.get('game'))
	};
}
