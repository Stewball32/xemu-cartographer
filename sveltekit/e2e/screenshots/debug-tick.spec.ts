// Tick debug tab smoke spec — exercises the M6c-style envelope header +
// Pretty/JSON Switch and the five Pretty sections (players / power_items /
// ctf_flags / game_globals / locals). Unlike the unit-N screenshot specs,
// this one runs assertions before the snapshot so a layout regression on
// the tick tab fails the suite even if the PNGs look superficially fine.

import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';

test('debug-tick: tick tab renders Pretty sections + envelope header + JSON view toggle', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(`/pod/${MOCK_INSTANCE_LIVE}/debug/#tick`);
	await page.waitForLoadState('networkidle');

	// Scope assertions to the active Tick tabpanel so labels that also live
	// in other tabs (e.g. game's 'seq') don't collide with the visibility
	// check.
	const tickPanel = page.getByRole('tabpanel', { name: 'Tick' });
	await expect(tickPanel).toBeVisible();

	// Envelope-stats header — the literal label text (exact match avoids
	// colliding with the "no tick envelope received yet" empty-state hint).
	await expect(tickPanel.getByText('tick envelope', { exact: true })).toBeVisible();
	for (const label of ['seq', 'received', 'v', 'instance']) {
		await expect(tickPanel.getByText(label, { exact: true }).first()).toBeVisible();
	}

	// Pretty-mode section headings — rendered as <code> elements with the
	// literal JSON-key strings. Every section renders even when empty.
	for (const heading of ['players', 'power_items', 'ctf_flags', 'game_globals', 'locals']) {
		await expect(tickPanel.locator('code', { hasText: new RegExp(`^${heading}$`) })).toBeVisible();
	}

	await page.screenshot({ path: 'e2e/screenshots/debug-tick-pretty.png', fullPage: true });

	// Flip the Pretty/JSON Switch and confirm the JSON view's TreeView card
	// renders. The mock setup doesn't pipe a real websocket envelope, so
	// the tree's empty-state message is what we assert lands — proving the
	// TickJson component mounted and the toggle wired through.
	await tickPanel.getByLabel('Toggle pretty / JSON view').click();
	await expect(tickPanel.getByText('tree', { exact: true })).toBeVisible();
	await expect(tickPanel.getByText('no envelope to walk')).toBeVisible();

	await page.screenshot({ path: 'e2e/screenshots/debug-tick-json.png', fullPage: true });
});
