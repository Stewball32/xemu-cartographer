// Playwright screenshot helper. Captures one full-page PNG per viewport for
// a route, so every spec produces consistent mobile/tablet/desktop evidence.
// Used by the canonical _example.spec.ts and copied verbatim by every per-unit
// spec. Workers should not modify this file.

import type { Page } from '@playwright/test';

type Viewport = { name: 'mobile' | 'tablet' | 'desktop'; width: number; height: number };

const VIEWPORTS: Viewport[] = [
	{ name: 'mobile', width: 375, height: 812 }, // iPhone 13-ish, mobile-first reference
	{ name: 'tablet', width: 768, height: 1024 }, // iPad portrait
	{ name: 'desktop', width: 1440, height: 900 } // typical laptop
];

// Captures the route at all three viewports. Output paths are stable per slug
// so PR diffs show a clean three-PNG group.
export async function screenshotPodRoute(page: Page, route: string, slug: string): Promise<void> {
	for (const vp of VIEWPORTS) {
		await page.setViewportSize({ width: vp.width, height: vp.height });
		await page.goto(route);
		// Wait for both network and the page's idle frame so reactive renders
		// settle before the snapshot.
		await page.waitForLoadState('networkidle');
		await page.screenshot({
			path: `e2e/screenshots/${slug}-${vp.name}.png`,
			fullPage: true
		});
	}
}
