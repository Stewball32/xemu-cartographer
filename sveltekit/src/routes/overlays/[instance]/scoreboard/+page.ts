import type { PageLoad } from './$types';
import { overlayParams } from '$lib/utils/overlay-params';

// Dynamic per-instance broadcast overlay — never prerendered (instance unknown at
// build, served via the SPA fallback). The M10 overlay token in ?token= is the
// credential (validated by the WS handshake), so the page loads anonymously.
export const prerender = false;

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	return overlayParams(params.instance, url);
};
