import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('unit-5-pod-probe: /pod/[name]/probe/ renders live diagnostics + ad-hoc probe tools', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await screenshotPodRoute(page, `/pod/${MOCK_INSTANCE_LIVE}/probe/`, 'unit-5-pod-probe');
});
