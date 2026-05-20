import { test } from '@playwright/test';
import { loginAsAdmin } from '../helpers/auth';
import { installPodMocks, MOCK_INSTANCE_LIVE } from '../helpers/mocks';
import { screenshotPodRoute } from '../helpers/screenshot';

test('unit-3-pod-view: /pod/[name]/view/ renders kiosk + controller + start/stop', async ({
	page
}) => {
	await installPodMocks(page);
	await loginAsAdmin(page);

	await screenshotPodRoute(page, `/pod/${MOCK_INSTANCE_LIVE}/view/`, 'unit-3-pod-view');
});
