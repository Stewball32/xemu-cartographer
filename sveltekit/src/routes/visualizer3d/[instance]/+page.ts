import type { PageLoad } from './$types';

// Dynamic per-instance 3D visualizer — sibling of /visualizer/[instance]/ (the
// 2D top-down daily-driver), fed by the SAME overlay WS feed. Never prerendered
// (instance is unknown at build, served via the SPA fallback). Like the OBS
// overlays + the 2D map, the M10 overlay token in ?token= IS the credential
// (validated by the WS handshake), so the page loads anonymously; ?mock=1
// previews with sample data and no token.
export const prerender = false;

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	const mockParam = url.searchParams.get('mock');
	return {
		instance: params.instance,
		token: url.searchParams.get('token') ?? '',
		mock: mockParam === '1' || mockParam === 'true',
		// Optional initial toggle for the static spawn-point reference layer.
		showSpawns: url.searchParams.get('spawns') === '1'
	};
};
