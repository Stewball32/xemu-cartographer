// Pure state helper for the M09 /play/ page. Keeps the "did the server resolve
// a match?" decision out of the Svelte component so it can be unit-tested.

export interface MeMatchResponse {
	/** Name of the container the caller's gamertag is currently in, or null. */
	container?: string | null;
}

export type PlayPhase = 'loading' | 'idle' | 'matched';

export interface PlayState {
	phase: PlayPhase;
	container: string | null;
}

export const INITIAL_PLAY_STATE: PlayState = { phase: 'loading', container: null };

/**
 * Reduce a /api/me/match response into the next play state. A null, empty, or
 * non-string container is the idle state (not in a match); a non-empty string
 * is a match. Defensive against malformed payloads so a bad response degrades
 * to idle rather than throwing.
 */
export function reducePlayState(resp: MeMatchResponse | null | undefined): PlayState {
	const container =
		resp && typeof resp.container === 'string' && resp.container.length > 0 ? resp.container : null;
	return { phase: container ? 'matched' : 'idle', container };
}
