import { describe, it, expect } from 'vitest';
import {
	STREAM_ASSETS,
	assetById,
	liveURL,
	mockPath,
	mockURL,
	MOCK_INSTANCE,
	type StreamAsset
} from './stream-assets';

const ORIGIN = 'https://cart.example';

describe('STREAM_ASSETS registry', () => {
	it('has the five existing surfaces + the two new ones, all with unique ids', () => {
		const ids = STREAM_ASSETS.map((a) => a.id);
		expect(new Set(ids).size).toBe(ids.length);
		for (const id of [
			'scoreboard',
			'status-strip',
			'timer',
			'killfeed',
			'players',
			'visualizer-2d',
			'visualizer-3d'
		]) {
			expect(ids).toContain(id);
		}
	});

	it('every entry produces a trailing-slash route path (SvelteKit trailingSlash=always)', () => {
		for (const a of STREAM_ASSETS) {
			const path = a.path('pod-a');
			expect(path.startsWith('/')).toBe(true);
			expect(path.endsWith('/')).toBe(true);
		}
	});

	it('every entry carries name, description, aspect, and an OBS hint', () => {
		for (const a of STREAM_ASSETS) {
			expect(a.name.length).toBeGreaterThan(0);
			expect(a.description.length).toBeGreaterThan(0);
			expect(a.aspect.w).toBeGreaterThan(0);
			expect(a.aspect.h).toBeGreaterThan(0);
			expect(a.aspect.label.length).toBeGreaterThan(0);
			expect(a.obsHint.length).toBeGreaterThan(0);
		}
	});

	it('assetById finds entries and misses cleanly', () => {
		expect(assetById('timer')?.name).toBe('Game timer');
		expect(assetById('nope')).toBeUndefined();
	});
});

describe('URL helpers', () => {
	const scoreboard = assetById('scoreboard') as StreamAsset;
	const players = assetById('players') as StreamAsset;

	it('mockPath uses the mock instance and ?mock=1', () => {
		expect(mockPath(scoreboard)).toBe(`/overlays/${MOCK_INSTANCE}/?mock=1`);
		expect(mockPath(scoreboard, 1.5)).toBe(`/overlays/${MOCK_INSTANCE}/?mock=1&scale=1.5`);
	});

	it('mockURL is the absolute form of mockPath', () => {
		expect(mockURL(ORIGIN, scoreboard)).toBe(`${ORIGIN}/overlays/${MOCK_INSTANCE}/?mock=1`);
	});

	it('liveURL carries the scoped token for token-scoped assets', () => {
		expect(liveURL(ORIGIN, scoreboard, 'pod-a', 'jwt.tok.en')).toBe(
			`${ORIGIN}/overlays/pod-a/?token=jwt.tok.en`
		);
	});

	it('liveURL omits the token for legacy JWT surfaces (players)', () => {
		expect(players.tokenScoped).toBe(false);
		expect(liveURL(ORIGIN, players, 'pod-a', 'jwt.tok.en')).toBe(`${ORIGIN}/overlays/players/`);
	});
});
