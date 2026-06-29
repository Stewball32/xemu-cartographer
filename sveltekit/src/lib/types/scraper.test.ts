import { describe, it, expect } from 'vitest';
import { localViewport } from './scraper';

// Mirrors the Go scraper.LocalViewport unit tests (internal/scraper/viewport_test.go)
// so the overlay-facing TS port stays in lockstep with the wire mapping.
describe('localViewport', () => {
	it('1 player → full screen', () => {
		expect(localViewport(1, 0)).toEqual({ x: 0, y: 0, w: 1, h: 1 });
	});

	it('2 players → horizontal halves (0 = top, 1 = bottom)', () => {
		const top = localViewport(2, 0);
		const bottom = localViewport(2, 1);
		expect(top).toEqual({ x: 0, y: 0, w: 1, h: 0.5 });
		expect(bottom).toEqual({ x: 0, y: 0.5, w: 1, h: 0.5 });
		// Full-width halves → horizontal split, NOT left/right.
		expect(top!.w).toBe(1);
		expect(bottom!.w).toBe(1);
	});

	it('3 players → 3 quadrants (bottom-right empty)', () => {
		expect(localViewport(3, 0)).toEqual({ x: 0, y: 0, w: 0.5, h: 0.5 });
		expect(localViewport(3, 1)).toEqual({ x: 0.5, y: 0, w: 0.5, h: 0.5 });
		expect(localViewport(3, 2)).toEqual({ x: 0, y: 0.5, w: 0.5, h: 0.5 });
	});

	it('4 players → full quadrants', () => {
		expect(localViewport(4, 0)).toEqual({ x: 0, y: 0, w: 0.5, h: 0.5 });
		expect(localViewport(4, 1)).toEqual({ x: 0.5, y: 0, w: 0.5, h: 0.5 });
		expect(localViewport(4, 2)).toEqual({ x: 0, y: 0.5, w: 0.5, h: 0.5 });
		expect(localViewport(4, 3)).toEqual({ x: 0.5, y: 0.5, w: 0.5, h: 0.5 });
	});

	it('out-of-range / non-local → null', () => {
		expect(localViewport(0, 0)).toBeNull(); // no local players
		expect(localViewport(2, 2)).toBeNull(); // index past count
		expect(localViewport(2, -1)).toBeNull(); // network/non-local marker
		expect(localViewport(5, 0)).toBeNull(); // count above the cap
	});
});
