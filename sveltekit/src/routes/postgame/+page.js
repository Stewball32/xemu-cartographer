// Native cartographer OBS source: subscribes to ONE instance's live feed
// (overlay token) and maps it to the pack's match/players shape via
// overlay-state. Instance-scoped + token-authed → dynamic (never prerendered),
// served via the SPA fallback like the POV overlay.
import { nativeOverlayParams } from '$lib/utils/overlay-state';

export const ssr = false;
export const prerender = false;

export function load({ url }) {
	return nativeOverlayParams(url);
}
