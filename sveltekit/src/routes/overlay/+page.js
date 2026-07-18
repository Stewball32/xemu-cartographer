import { overlayParams } from '$lib/overlay/params.js';
export const ssr = false;
export function load({ url }) {
	return overlayParams(url);
}
