import type { PageLoad } from './$types';

// Dynamic per-instance visualizer — never prerendered (instance is unknown at
// build, served via the SPA fallback). Like the OBS overlays, the M10 overlay
// token in ?token= IS the credential (validated by the WS handshake), so the
// page loads anonymously; ?mock=1 previews with sample data and no token.
export const prerender = false;

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	const mockParam = url.searchParams.get('mock');
	return {
		instance: params.instance,
		token: url.searchParams.get('token') ?? '',
		mock: mockParam === '1' || mockParam === 'true',
		// Per-map demo: render the named map's cached geometry populated with mock
		// players placed within its real world-bounds (no live feed / token). Proves
		// the pipeline on any extracted map. e.g. ?map=chillout
		map: url.searchParams.get('map') ?? '',
		// Optional initial toggle for the static spawn-point reference layer.
		showSpawns: url.searchParams.get('spawns') === '1'
	};
};
