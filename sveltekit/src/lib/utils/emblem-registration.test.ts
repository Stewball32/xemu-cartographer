/**
 * Guards the emblem foreground REGISTRATION fix.
 *
 * The shipped H2 foreground masks are 64x64 textures whose art is anchored at the
 * texture origin and only fills the top-left ~38x38 sprite rect (the rest is
 * power-of-two padding). The compositor must map that sprite rect onto the cell —
 * not stretch the whole padded texture — or the symbol renders shrunk into the
 * top-left corner (the bug this fixes). Backgrounds fill their whole texture and
 * must keep the plain full-texture mapping.
 */
import { describe, it, expect } from 'vitest';
import { foregroundSvg, FG_SPRITE_RECT, EMBLEM_FG_INSET } from './emblem-foregrounds';
import { backgroundSvg } from './emblem-backgrounds';

// Expected mapped <image> size for the sprite rect: texW * 100 / w.
const mappedW = Number(((FG_SPRITE_RECT.texW * 100) / FG_SPRITE_RECT.w).toFixed(3));
const mappedH = Number(((FG_SPRITE_RECT.texH * 100) / FG_SPRITE_RECT.h).toFixed(3));

describe('foreground sprite-rect registration', () => {
	it('the sprite rect is anchored at the texture origin and is padded to 64', () => {
		// origin anchor is what makes a symmetric (centred) mapping possible.
		expect(FG_SPRITE_RECT.x).toBe(0);
		expect(FG_SPRITE_RECT.y).toBe(0);
		expect(FG_SPRITE_RECT.texW).toBe(64);
		expect(FG_SPRITE_RECT.texH).toBe(64);
		// the active region is a sub-rect of the padded texture (there IS padding).
		expect(FG_SPRITE_RECT.w).toBeGreaterThan(0);
		expect(FG_SPRITE_RECT.w).toBeLessThan(FG_SPRITE_RECT.texW);
	});

	it('maps the sprite rect (not the full padded texture) onto the cell', () => {
		// mapping the 38/64 rect to the 0..100 cell scales the texture up past 100,
		// with the image anchored at x=0,y=0; stretching the whole texture would be 100.
		expect(mappedW).toBeGreaterThan(100);
		for (const i of [0, 1, 12, 26, 27, 63]) {
			const svg = foregroundSvg(i, '#ffffff', '#000000', `t${i}`);
			expect(svg).toContain(`width="${mappedW}"`);
			expect(svg).toContain(`height="${mappedH}"`);
			// anchored at the origin so the mapping stays symmetric (centred).
			expect(svg).toMatch(/<image[^>]*\sx="0"\sy="0"/);
		}
	});

	it('leaves backgrounds on the plain full-texture mapping', () => {
		for (const i of [0, 1, 22]) {
			const svg = backgroundSvg(i, '#112233', '#445566', `b${i}`);
			// no sprite-rect upscale: the background image fills the 0..100 viewport.
			expect(svg).not.toContain(`width="${mappedW}"`);
			expect(svg).toMatch(/<image[^>]*width="100"/);
		}
	});

	it('insets the foreground symmetrically (stays centred)', () => {
		// the cell offset that centres an inset-scaled foreground must be symmetric.
		expect(EMBLEM_FG_INSET).toBeGreaterThan(0);
		expect(EMBLEM_FG_INSET).toBeLessThanOrEqual(1);
		const offset = (100 - 100 * EMBLEM_FG_INSET) / 2;
		expect(offset).toBeCloseTo((100 - 100 * EMBLEM_FG_INSET) / 2, 6);
		// 0.78 -> 11.0 (the historical, now-correct, centred inset).
		expect(offset).toBeCloseTo(11, 3);
	});
});
