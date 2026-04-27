export const HIDDEN_LAYOUT_PATHS: readonly string[] = [
	// Overlays render in OBS Browser Source — header/nav/toaster chrome must be
	// suppressed so the route is the only thing on screen, with a transparent
	// background for compositing.
	'/overlays/'
];

export function isLayoutHidden(pathname: string): boolean {
	return HIDDEN_LAYOUT_PATHS.some((p) => pathname.startsWith(p));
}
