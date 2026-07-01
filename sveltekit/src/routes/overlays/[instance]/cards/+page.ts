import type { PageLoad } from './$types';
import { overlayParams } from '$lib/utils/overlay-params';

// Dynamic per-instance broadcast overlay — see scoreboard/+page.ts for the
// no-prerender / token-as-credential rationale.
export const prerender = false;

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	return overlayParams(params.instance, url);
};
