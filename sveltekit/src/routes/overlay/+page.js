// The POV overlay is a cartographer-native OBS source: it targets a console by
// name and derives its splitscreen layout from the live feed (see overlay-split).
// Dynamic (never prerendered), served via the SPA fallback like the other three
// overlays.
//
// Shares nativeOverlayParams with scorebug/leaderboard/postgame so all four take
// the same URL shape; only ?layout is POV-specific.
import { nativeOverlayParams } from '$lib/utils/overlay-state';

export const ssr = false;
export const prerender = false;

export function load({ url }) {
	const layout = url.searchParams.get('layout');
	return {
		...nativeOverlayParams(url),
		// Splitscreen is AUTO-detected from the live feed. ?layout=1..4 is an
		// optional manual override for testing only; 0 = auto (the default).
		layoutOverride: layout ? Number(layout) : 0
	};
}
