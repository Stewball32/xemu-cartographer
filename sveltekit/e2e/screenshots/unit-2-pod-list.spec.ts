// Unit 2: /pod/ list page screenshot spec.

import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('unit-2-pod-list: /pod/ renders with mocked admin auth + fixture data', async ({ page }) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	// Demonstrates the per-name URL pattern by including MOCK_INSTANCE_LIVE,
	// which other specs will reuse for /pod/<name>/view/, /pod/<name>/debug/,
	// and /pod/<name>/probe/.
	void MOCK_INSTANCE_LIVE;

	await screenshotPodRoute(page, '/pod/', 'unit-2-pod-list');
});
