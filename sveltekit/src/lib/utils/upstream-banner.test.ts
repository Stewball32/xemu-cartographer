import { describe, expect, it } from 'vitest';
import { upstreamBannerText } from './upstream-banner';

const at = (iso: string) => `T(${iso})`;

describe('upstreamBannerText', () => {
	it('is silent in-process and while a wire stream is healthy', () => {
		expect(upstreamBannerText(null, at)).toBeNull();
		expect(upstreamBannerText({ mode: 'in-process' }, at)).toBeNull();
		expect(
			upstreamBannerText({ mode: 'wire', connected: true, since: '2026-09-14T10:00:00Z' }, at)
		).toBeNull();
	});

	it('names the disconnect time, the attempt count and the last error', () => {
		expect(
			upstreamBannerText(
				{
					mode: 'wire',
					connected: false,
					since: '2026-09-14T10:00:00Z',
					attempts: 3,
					last_error: 'dial tcp 127.0.0.1:8990: connect: connection refused'
				},
				at
			)
		).toBe(
			'Scraper upstream disconnected since T(2026-09-14T10:00:00Z) — reconnecting (3 attempts, last: dial tcp 127.0.0.1:8990: connect: connection refused)…'
		);
		expect(
			upstreamBannerText(
				{ mode: 'wire', connected: false, since: '2026-09-14T10:00:00Z', attempts: 1 },
				at
			)
		).toBe(
			'Scraper upstream disconnected since T(2026-09-14T10:00:00Z) — reconnecting (1 attempt)…'
		);
		expect(upstreamBannerText({ mode: 'wire', connected: false }, at)).toBe(
			'Scraper upstream disconnected since unknown — reconnecting (0 attempts)…'
		);
	});

	it('flags a rejected feed token even while the socket is up', () => {
		expect(
			upstreamBannerText(
				{ mode: 'wire', connected: true, auth_rejected: true, since: '2026-09-14T10:00:00Z' },
				at
			)
		).toBe(
			"Scraper upstream rejected the feed token (connected since T(2026-09-14T10:00:00Z), nothing streams) — check XC_SCRAPER_TOKEN against the daemon's --token."
		);
		expect(
			upstreamBannerText(
				{ mode: 'wire', connected: false, auth_rejected: true, since: '2026-09-14T10:00:00Z' },
				at
			)
		).toBe(
			"Scraper upstream rejected the feed token (since T(2026-09-14T10:00:00Z)) — check XC_SCRAPER_TOKEN against the daemon's --token."
		);
	});
});
