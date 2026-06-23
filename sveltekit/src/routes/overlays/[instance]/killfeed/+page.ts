import type { PageLoad } from './$types';

// Kill-feed overlay — a transparent OBS Browser Source bound to the same scoped
// overlay token as the other surfaces. It rides the read-only `host:<instance>:event`
// per-class room (a normal class join the overlay token may make — distinct from
// the rejected request_events backfill). Never prerendered (instance unknown at
// build, served via the SPA fallback); ?mock=1 previews a rolling kill feed with
// no token.
export const prerender = false;

function parseScale(raw: string | null): number {
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) return 1;
	return Math.min(3, Math.max(0.5, n));
}

function parseMax(raw: string | null): number {
	const n = Number(raw);
	if (!Number.isFinite(n) || n <= 0) return 5;
	return Math.min(12, Math.max(1, Math.floor(n)));
}

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	const mockParam = url.searchParams.get('mock');
	return {
		instance: params.instance,
		token: url.searchParams.get('token') ?? '',
		mock: mockParam === '1' || mockParam === 'true',
		scale: parseScale(url.searchParams.get('scale')),
		max: parseMax(url.searchParams.get('max'))
	};
};
