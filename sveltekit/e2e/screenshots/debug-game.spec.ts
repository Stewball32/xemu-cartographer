// Debug-page Game-tab smoke + screenshot. Navigates to /pod/debug/<live>/#game,
// activates the Game tab, and asserts the rewritten tab renders its
// envelope-stats header tiles, every Pretty section heading, and the JSON
// view's TreeView after toggling the Switch.
//
// The WS upgrade is not mocked, so the game-class envelope never arrives —
// that is intentional. The Pretty view renders every section with `—`
// placeholders (the tab's promise is "always show the structure"), and the
// JSON view's TreeView only renders when an envelope exists, so the Switch
// click verifies the toggle wiring even though the tree body stays empty.

import { test, expect } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('debug-game: /pod/debug/[name]/#game renders Pretty/JSON game tab', async ({ page }) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.goto(`/pod/debug/${MOCK_INSTANCE_LIVE}/#game`);
	await page.waitForLoadState('networkidle');

	// Hash-driven tab selection happens during onMount; nudge via the
	// trigger to be deterministic across viewports.
	await page.getByRole('tab', { name: 'Game', exact: true }).click();

	// Skeleton/Zag Tabs.Content marks the active panel id with the tab
	// value suffix (e.g. tabs:c4:content-game). Scope all assertions to
	// that panel so we never pick up the xbox tab's hidden duplicate of
	// the same StatTile labels.
	const gameTab = page.locator('[role="tabpanel"][id$=":content-game"]');
	await expect(gameTab).toBeVisible();

	// Envelope-stats header — these labels live in StatTile <label> spans and
	// stay visible regardless of whether a WS payload has arrived.
	const headerLabels = ['seq', 'tick', 'received', 'v', 'instance'];
	for (const label of headerLabels) {
		await expect(gameTab.getByText(label, { exact: true })).toBeVisible();
	}

	// Pretty-view section headings (literal JSON keys, rendered inside <code>).
	const sectionIds = ['top-level', 'config', 'team_scores', 'players', 'machines', 'network'];
	for (const id of sectionIds) {
		await expect(gameTab.locator(`code:text-is("${id}")`)).toBeVisible();
	}

	// Toggle the Pretty/JSON switch — Skeleton's Switch renders a hidden input
	// keyed by the aria-label we set on the wrapper.
	await gameTab.getByLabel('Toggle pretty / JSON view', { exact: true }).click();

	// JSON view: "tree" header label is always rendered, even before an
	// envelope arrives (the empty-state copy lives inside the same card).
	await expect(gameTab.getByText('tree', { exact: true })).toBeVisible();

	await screenshotPodRoute(page, `/pod/debug/${MOCK_INSTANCE_LIVE}/#game`, 'debug-game');
});
