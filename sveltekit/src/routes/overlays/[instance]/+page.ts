import type { PageLoad } from './$types';

// Dynamic per-instance overlay — never prerendered (instance is unknown at
// build, served via the SPA fallback). No auth redirect: the M10 overlay token
// in ?token= IS the credential and is validated by the WS handshake, so the
// page itself loads anonymously (an OBS browser source carries no PB session).
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
