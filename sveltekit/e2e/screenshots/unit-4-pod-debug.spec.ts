import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('unit-4: /pod/[name]/debug/ renders multi-tab inspector with mocked data', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await screenshotPodRoute(page, `/pod/${MOCK_INSTANCE_LIVE}/debug/`, 'unit-4-pod-debug');
});
