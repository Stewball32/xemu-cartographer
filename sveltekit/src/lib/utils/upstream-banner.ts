import type { UpstreamStatus } from '$lib/types/scraper';

// upstreamBannerText is the studio "upstream disconnected" banner
// (DESIGN-STEP8 §12) as one line, or null when there is nothing to show:
// in-process mode has no upstream, and a connected stream whose token the
// daemon accepted is the normal state. `formatTime` renders the `since`
// timestamp (injectable so tests are locale-independent).
export function upstreamBannerText(
	status: UpstreamStatus | null | undefined,
	formatTime: (iso: string) => string = defaultFormatTime
): string | null {
	if (!status || status.mode !== 'wire') return null;
	const since = status.since ? formatTime(status.since) : 'unknown';
	if (status.auth_rejected) {
		const when = status.connected ? `connected since ${since}, nothing streams` : `since ${since}`;
		return `Scraper upstream rejected the feed token (${when}) — check XC_SCRAPER_TOKEN against the daemon's --token.`;
	}
	if (status.connected) return null;
	const attempts = status.attempts ?? 0;
	const tries = `${attempts} ${attempts === 1 ? 'attempt' : 'attempts'}`;
	const last = status.last_error ? `, last: ${status.last_error}` : '';
	return `Scraper upstream disconnected since ${since} — reconnecting (${tries}${last})…`;
}

function defaultFormatTime(iso: string): string {
	const d = new Date(iso);
	return Number.isNaN(d.getTime()) ? iso : d.toLocaleTimeString();
}
