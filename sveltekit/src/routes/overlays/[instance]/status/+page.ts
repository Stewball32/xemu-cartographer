import type { PageLoad } from './$types';

// Match-status strip — same dynamic-instance / overlay-token contract as the
// scoreboard at /overlays/[instance]/. Reuses the minted token (the token is
// scoped to the instance, not the view), so the URL is just the scoreboard URL
// with `/status` inserted before the query string.
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
