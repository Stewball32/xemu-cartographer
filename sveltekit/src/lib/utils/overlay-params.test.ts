import { describe, it, expect } from 'vitest';
import { parseScale, overlayParams } from './overlay-params';

describe('parseScale', () => {
	it('defaults to 1 for missing / invalid / non-positive input', () => {
		expect(parseScale(null)).toBe(1);
		expect(parseScale('')).toBe(1);
		expect(parseScale('abc')).toBe(1);
		expect(parseScale('0')).toBe(1);
		expect(parseScale('-2')).toBe(1);
	});

	it('clamps to the OBS range 0.5..3', () => {
		expect(parseScale('0.1')).toBe(0.5);
		expect(parseScale('5')).toBe(3);
		expect(parseScale('1.5')).toBe(1.5);
	});
});

describe('overlayParams', () => {
	it('projects the path param + query into the shared bundle', () => {
		const url = new URL('http://x/overlays/pod1/scoreboard/?token=abc&mock=1&scale=2&game=h2');
		expect(overlayParams('pod1', url)).toEqual({
			instance: 'pod1',
			token: 'abc',
			mock: true,
			scale: 2,
			game: 'h2'
		});
	});

	it('defaults token/mock/scale/game when absent', () => {
		const url = new URL('http://x/overlays/pod1/cards/');
		expect(overlayParams('pod1', url)).toEqual({
			instance: 'pod1',
			token: '',
			mock: false,
			scale: 1,
			game: 'ce'
		});
	});

	it('accepts mock=true as well as mock=1', () => {
		const url = new URL('http://x/o/?mock=true');
		expect(overlayParams('i', url).mock).toBe(true);
	});
});
