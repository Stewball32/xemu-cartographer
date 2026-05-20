import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('debug-events: /pod/[name]/debug/#events renders header + feed and toggles JSON', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.goto(`/pod/${MOCK_INSTANCE_LIVE}/debug/#events`);
	await page.waitForLoadState('networkidle');

	// Envelope-stats header tiles render (even when the mocked WS hands us
	// no live event envelopes — they fall back to '—'). The "logged" tile is
	// unique to this header so it doubles as a marker for "header mounted".
	await expect(page.getByText('events log').first()).toBeVisible();
	const loggedTile = page.getByText('logged', { exact: true }).first();
	await expect(loggedTile).toBeVisible();

	// Existing Pretty body still renders the feed section ("Event feed" is
	// the EventFeed section header that survives the rewrite).
	await expect(page.getByText('Event feed').first()).toBeVisible();

	// Click the Pretty/JSON Switch. Skeleton renders the label, an icon, and
	// a HiddenInput together — the wrapping <label> isn't visible, so click
	// the underlying checkbox via force.
	const toggle = page.getByRole('checkbox', { name: 'Pretty' }).first();
	await toggle.click({ force: true });

	// After toggling, the JSON-view mounts: the Reset button + the empty-tree
	// hint render alongside the "json" column heading. Targeting the Reset
	// button avoids brittle title-case text matches that could collide with
	// "TreeView" / "treeNode" strings rendered elsewhere.
	await expect(page.getByRole('button', { name: 'Reset' })).toBeVisible();
	await expect(page.getByText('no event envelopes to walk')).toBeVisible();

	// Final screenshot: capture the route at every viewport so the PR carries
	// visual evidence the new header / view-toggle render across breakpoints.
	await screenshotPodRoute(page, `/pod/${MOCK_INSTANCE_LIVE}/debug/#events`, 'debug-events');
});
