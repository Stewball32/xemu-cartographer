import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
