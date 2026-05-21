import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

// Modeled on unit-4-pod-debug.spec.ts but scoped to the per-envelope-class
// debug tab (route hash `#debug`). The mock fixtures don't simulate a live
// WebSocket so the envelope-stats header tiles render placeholder values
// (no envelope received yet) — we still assert the section headings exist
// because they're always present in the Pretty view, and we still exercise
// the Pretty/JSON Switch + TreeView shell to catch render-time regressions.
test('debug-debug: /admin/pod/[name]/debug/#debug renders envelope-stats header + Pretty sections + JSON view', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(`/admin/pod/${MOCK_INSTANCE_LIVE}/debug/#debug`);
	await page.waitForLoadState('networkidle');

	// Skeleton Tabs keeps every tabpanel in the DOM, only the selected one is
	// visible. Scope every locator to the active Debug tabpanel so we don't
	// accidentally pick up the same labels from the sibling (hidden) Xbox tab.
	const debugPanel = page.getByRole('tabpanel', { name: 'Debug' });
	await expect(debugPanel).toBeVisible();

	// envelope-stats header — every tile is always present so labels are
	// stable even with no envelope on the wire yet.
	for (const label of ['seq', 'tick', 'received', 'v', 'instance']) {
		await expect(debugPanel.getByText(label, { exact: true }).first()).toBeVisible();
	}

	// Pretty section heading (the literal JSON key, rendered as <code>).
	// state_inputs / score_probe used to live on the debug envelope but moved
	// to the on-demand probe envelope in M6c — only `players` remains here.
	await expect(debugPanel.locator('code', { hasText: 'players' }).first()).toBeVisible();

	// Flip the Pretty/JSON Switch — Skeleton's Switch projects as role=checkbox
	// with the current-state label ("Pretty" / "JSON") as the accessible name.
	// The hidden input only takes pointer events through its visible label, so
	// click the labelling container instead.
	await debugPanel.getByLabel(/toggle pretty.*json/i).click();

	// JSON view renders the tree shell card; without an envelope, the
	// "no envelope to walk" empty-state copy is fine to assert.
	await expect(debugPanel.getByText(/no envelope to walk/i).first()).toBeVisible();

	await screenshotPodRoute(page, `/admin/pod/${MOCK_INSTANCE_LIVE}/debug/#debug`, 'debug-debug');
});
