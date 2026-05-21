// Canonical /admin/pod/* screenshot spec. Workers copy this file, rename to
// <unit-slug>.spec.ts, and change the route + slug. Don't touch the helpers.

import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('_example: /admin/pod/ renders with mocked admin auth + fixture data', async ({ page }) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	// Demonstrates the per-name URL pattern by including MOCK_INSTANCE_LIVE,
	// which other specs will reuse for /admin/pod/<name>/view/, /admin/pod/<name>/debug/,
	// and /admin/pod/<name>/probe/.
	void MOCK_INSTANCE_LIVE;

	await screenshotPodRoute(page, '/admin/pod/', '_example-pod-list');
});
