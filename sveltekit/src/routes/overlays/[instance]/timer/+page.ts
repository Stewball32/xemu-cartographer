import type { PageLoad } from './$types';

// Game-timer / match-clock overlay — a transparent OBS Browser Source sibling of
// the scoreboard + status strip, bound to the same scoped overlay token (the
// token scopes a host:<instance> room, not a view). Never prerendered (instance
// unknown at build, served via the SPA fallback). The M10 overlay token in
// ?token= IS the credential (validated by the WS handshake), so the page loads
// anonymously; ?mock=1 previews with sample data and no token.
export const prerender = false;

function parseScale(raw: string | null): number {
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) return 1;
	return Math.min(3, Math.max(0.5, n));
}

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	const mockParam = url.searchParams.get('mock');
	return {
		instance: params.instance,
		token: url.searchParams.get('token') ?? '',
		mock: mockParam === '1' || mockParam === 'true',
		scale: parseScale(url.searchParams.get('scale'))
	};
};
