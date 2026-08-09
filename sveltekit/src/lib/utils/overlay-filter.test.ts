import { describe, it, expect } from 'vitest';
import { filterRoster, type DummyFilterConfig } from './overlay-filter';

// Mirrors the live beta-stream neutral-host case: the host's local seat is a
// dummy (is_local=true) plus a globally allowlisted gamertag; real remote
// players must survive. Byte-for-byte with internal/scraper/roster.FilterRoster.
type P = { name: string; is_local?: boolean | null };
const roster: P[] = [
	{ name: 'Crazy', is_local: true }, // neutral-host local dummy
	{ name: 'OG50 II', is_local: false }, // real remote
	{ name: 'Stewball32', is_local: false }, // real remote
	{ name: 'BenchBot', is_local: false } // globally allowlisted dummy
];

describe('filterRoster (client port of roster.FilterRoster)', () => {
	it('null config is a no-op (poll path / unconfigured)', () => {
		expect(filterRoster(roster, null)).toEqual(roster);
		expect(filterRoster(roster, undefined)).toBe(roster);
	});

	it('neutral host drops the local seat, keeps real remotes', () => {
		const cfg: DummyFilterConfig = { is_neutral_host: true, dummy_gamertags: [] };
		expect(filterRoster(roster, cfg).map((p) => p.name)).toEqual([
			'OG50 II',
			'Stewball32',
			'BenchBot'
		]);
	});

	it('allowlisted gamertag dropped regardless of host flag (case/space-insensitive)', () => {
		const cfg: DummyFilterConfig = { is_neutral_host: false, dummy_gamertags: ['  benchBOT '] };
		expect(filterRoster(roster, cfg).map((p) => p.name)).toEqual([
			'Crazy',
			'OG50 II',
			'Stewball32'
		]);
	});

	it('neutral host + allowlist together: only real remotes remain', () => {
		const cfg: DummyFilterConfig = { is_neutral_host: true, dummy_gamertags: ['benchbot'] };
		expect(filterRoster(roster, cfg).map((p) => p.name)).toEqual(['OG50 II', 'Stewball32']);
	});

	it('never drops a non-local real player just because a dummy list is present', () => {
		const cfg: DummyFilterConfig = { is_neutral_host: true, dummy_gamertags: ['someone-else'] };
		const kept = filterRoster(roster, cfg).map((p) => p.name);
		expect(kept).toContain('OG50 II');
		expect(kept).toContain('Stewball32');
	});

	it('is pure — does not mutate the input', () => {
		const copy = [...roster];
		filterRoster(roster, { is_neutral_host: true, dummy_gamertags: ['benchbot'] });
		expect(roster).toEqual(copy);
	});
});
