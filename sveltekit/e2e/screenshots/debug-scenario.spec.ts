// Scenario debug tab — verifies the migrated xbox-pattern Pretty/JSON views
// render against the mocked /pod/* fixture set. Walks the envelope-stats
// header, every Pretty Accordion section heading, and the post-toggle
// TreeView before capturing a desktop screenshot.

import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';

const ENVELOPE_TILES = ['seq', 'tick', 'received', 'v', 'instance'] as const;

const PRETTY_SECTIONS = [
	'top-level',
	'fog',
	'memory_regions',
	'object_types',
	'player_spawns',
	'power_item_spawns',
	'tag_defs'
] as const;

test('debug-scenario: Pretty/JSON tab renders headings + tree on mocked instance', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(`/pod/debug/${MOCK_INSTANCE_LIVE}/#scenario`);
	await page.waitForLoadState('networkidle');

	// Skeleton Tabs.Content keeps every panel mounted but hides inactive
	// ones; scoping assertions to the active panel avoids matching the
	// xbox tab's twin envelope-stats header that also renders 'seq' etc.
	// Skeleton's Tabs uses `[hidden]` to hide inactive panels, so a
	// `[role=tabpanel]:not([hidden])` filter resolves to the visible one.
	const panel = page.locator('[role="tabpanel"]:not([hidden])').first();
	await expect(panel).toBeVisible();

	// Pretty is the default viewMode; the envelope-stats header always
	// renders even before the first envelope arrives, so the tile labels
	// are a stable assertion against the static bundle.
	for (const label of ENVELOPE_TILES) {
		await expect(panel.getByText(label, { exact: true }).first()).toBeVisible();
	}

	// Each Accordion section trigger renders the literal JSON key as a
	// monospace code element so a future relabel won't silently break the
	// JSON-mirroring contract.
	for (const id of PRETTY_SECTIONS) {
		await expect(panel.locator(`code:has-text("${id}")`).first()).toBeVisible();
	}

	// Flip the Switch to JSON. The Skeleton Switch sets aria-label="Toggle
	// pretty / JSON view" on the root element; targeting that label keeps
	// the test resilient to thumb/control class changes. Use {force: true}
	// because the visible thumb sits over the underlying input.
	await panel.getByLabel('Toggle pretty / JSON view').click({ force: true });

	// The JSON pane is keyed by a "tree" label and a Reset button; either
	// is sufficient to confirm the view swap landed. We don't assert against
	// envelope-derived data because the static bundle never receives one
	// (no live WebSocket).
	await expect(panel.getByText('tree', { exact: true })).toBeVisible();

	await page.screenshot({
		path: 'e2e/screenshots/debug-scenario-desktop.png',
		fullPage: true
	});
});
