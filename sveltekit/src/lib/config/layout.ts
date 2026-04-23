export const HIDDEN_LAYOUT_PATHS: readonly string[] = [
	// "/path/",
];

export function isLayoutHidden(pathname: string): boolean {
	return HIDDEN_LAYOUT_PATHS.some((p) => pathname.startsWith(p));
}
