import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('unit-5-pod-probe: /pod/probe/[name]/ renders live diagnostics + ad-hoc probe tools', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await screenshotPodRoute(page, `/pod/probe/${MOCK_INSTANCE_LIVE}/`, 'unit-5-pod-probe');
});
