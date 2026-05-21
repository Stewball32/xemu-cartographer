import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

// Companion to unit-4-pod-debug.spec.ts that focuses on the objects tab
// (M6c migration to the xbox Pretty/JSON pattern). Loads the debug page
// with `#objects` so the deep-linked tab opens, asserts the envelope-stats
// header tiles + Pretty section headings render, flips the Pretty/JSON
// Switch and asserts the TreeView mounts, then snapshots all viewports.

test('debug-objects: /admin/pod/[name]/debug/#objects renders Pretty + JSON views', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(`/admin/pod/${MOCK_INSTANCE_LIVE}/debug/#objects`);
	await page.waitForLoadState('networkidle');

	// Skeleton Tabs renders every Tabs.Content in the DOM (hidden tabs are
	// CSS-hidden, not unmounted), so scoping the assertions to the active
	// "Objects" tabpanel keeps us from matching `seq`/`tick` labels that
	// exist in the (still-mounted) Xbox stats header.
	const panel = page.getByRole('tabpanel', { name: 'Objects' });

	// Envelope-stats header tiles — labels are stable text inside StatTile,
	// rendered as small-caps spans.
	for (const label of ['seq', 'tick', 'received', 'v', 'instance']) {
		await expect(panel.getByText(label, { exact: true })).toBeVisible();
	}

	// Pretty-view section headings render even before any envelope arrives —
	// the Accordion renders every section regardless of payload.
	await expect(panel.locator('code', { hasText: 'objects' })).toBeVisible();
	await expect(panel.locator('code', { hasText: 'projectiles' })).toBeVisible();

	// Flip the Pretty/JSON toggle and confirm the JSON view mounts. Skeleton's
	// Switch primitive renders an accessible wrapper labelled "Toggle pretty
	// / JSON view"; the hidden input inside is a checkbox. Click the wrapper
	// to flip viewMode → 'json' on the page-level debug context. The JSON
	// view always renders `tree` + `json` card titles; the TreeView itself
	// only mounts once an envelope arrives (no WS mocks in this spec so the
	// tree card shows the "no envelope to walk" placeholder).
	const toggle = panel.getByLabel('Toggle pretty / JSON view');
	await toggle.click();
	await expect(panel.getByText('tree', { exact: true })).toBeVisible();
	await expect(panel.getByText('json', { exact: true })).toBeVisible();
	await expect(panel.getByText('no envelope to walk')).toBeVisible();

	// Snapshot all three viewports (mobile / tablet / desktop).
	await screenshotPodRoute(
		page,
		`/admin/pod/${MOCK_INSTANCE_LIVE}/debug/#objects`,
		'debug-objects'
	);
});
