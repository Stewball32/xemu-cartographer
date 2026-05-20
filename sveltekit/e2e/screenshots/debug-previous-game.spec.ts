// Screenshot spec for the rewritten Previous-Game debug tab. Walks the
// envelope-class layout (envelope-stats header + `top-level` / `game` /
// `events` Accordion sections), exercises the Pretty ↔ JSON toggle, and
// captures one PNG per viewport.

import { expect, test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('debug-previous-game: /pod/[name]/debug/#previous_game renders envelope-class tab', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await page.goto(`/pod/${MOCK_INSTANCE_LIVE}/debug/#previous_game`);
	await page.waitForLoadState('networkidle');

	// Scope every assertion to the active tab's panel — Skeleton keeps
	// every Tabs.Content mounted, so other tabs (Xbox) also have stat
	// tiles labelled `seq` etc. in the DOM.
	const panel = page.getByRole('tabpanel', { name: 'Previous Game' });

	// Envelope-stats header: the label "previous_game envelope" plus
	// five stat tiles (seq/tick/received/v/instance). Text-only assertions
	// here would clash with the empty-state hint "no previous_game envelope
	// received yet" (substring overlap), so use exact-match for the tile
	// labels we expect.
	await expect(panel.getByText('previous_game envelope').first()).toBeVisible();
	await expect(panel.getByText('seq', { exact: true })).toBeVisible();
	await expect(panel.getByText('received', { exact: true })).toBeVisible();
	await expect(panel.getByText('instance', { exact: true })).toBeVisible();

	// Pretty view default — Accordion section headings render literal JSON
	// keys via <code>. The headings are uppercase via CSS but the text node
	// itself is lowercase, so we assert on the lowercase form.
	await expect(panel.getByRole('button', { name: 'top-level' })).toBeVisible();
	await expect(panel.getByRole('button', { name: /^game/ })).toBeVisible();
	await expect(panel.getByRole('button', { name: /^events/ })).toBeVisible();

	// Flip to JSON view via the Switch in the envelope-stats header.
	// Skeleton's Switch wraps the interactive checkbox in a clickable
	// container whose aria-label is "Toggle pretty / JSON view"; click the
	// wrapper to avoid pointer-events being intercepted by the inner label
	// span.
	await panel.getByLabel('Toggle pretty / JSON view').click();

	// JSON view: the Tree panel renders an envelope walk. The tree always
	// exposes envelope-level scalars (v, type, instance, seq, tick, ts, data)
	// regardless of whether the payload is null.
	await expect(panel.getByText('tree', { exact: true })).toBeVisible();

	await screenshotPodRoute(
		page,
		`/pod/${MOCK_INSTANCE_LIVE}/debug/#previous_game`,
		'debug-previous-game'
	);
});
