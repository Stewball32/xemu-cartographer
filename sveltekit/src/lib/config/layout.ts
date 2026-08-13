export const HIDDEN_LAYOUT_PATHS: readonly string[] = [
	// OBS browser sources render standalone — header/nav/toaster chrome must be
	// suppressed so the route is the only thing on screen, with a transparent
	// background for compositing. One entry per route in the
	// LAN_OBS_Browser_Sources pack (the /studio page is the catalog that lists
	// them and DOES keep app chrome).
	'/overlay/',
	'/scorebug/',
	'/leaderboard/',
	'/postgame/'
];

export function isLayoutHidden(pathname: string): boolean {
	return HIDDEN_LAYOUT_PATHS.some((p) => pathname.startsWith(p));
}
