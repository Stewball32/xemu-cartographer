// The POV overlay is a cartographer-native OBS source: it subscribes to ONE
// instance's live feed and derives its splitscreen layout server-side (see
// overlay-split). Instance-scoped + token-authed, so it's dynamic (never
// prerendered) and served via the SPA fallback like the visualizers.
export const ssr = false;
export const prerender = false;

export function load({ url }) {
	const p = url.searchParams;
	const mock = p.get('mock');
	const layout = p.get('layout');
	return {
		// PoC: target by console name alone (no instance/token) — the feed
		// resolves it to whichever host currently sees that console.
		console: p.get('console') ?? '',
		// The container/instance whose live split to render, + its overlay token.
		instance: p.get('instance') ?? '',
		token: p.get('token') ?? '',
		// ?mock=1 previews with sample data (no token needed).
		mock: mock === '1' || mock === 'true',
		// Optional display-name overrides: ?names=SCRAPED:Display,…
		names: Object.fromEntries(
			(p.get('names') ?? '')
				.split(',')
				.filter(Boolean)
				.map((pair) => pair.split(':'))
		),
		// Splitscreen is AUTO-detected from the live feed. ?layout=1..4 is an
		// optional manual override for testing only; 0 = auto (the default).
		layoutOverride: layout ? Number(layout) : 0
	};
}
