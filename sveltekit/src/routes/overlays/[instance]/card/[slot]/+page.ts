import type { PageLoad } from './$types';
import { overlayParams } from '$lib/utils/overlay-params';

// Single-player "spotlight" card, keyed by roster slot (player index 0..15).
export const prerender = false;

export const load: PageLoad = async ({ params, url, parent }) => {
	await parent();
	const slot = Number.parseInt(params.slot, 10);
	return {
		...overlayParams(params.instance, url),
		slot: Number.isFinite(slot) && slot >= 0 ? slot : 0
	};
};
