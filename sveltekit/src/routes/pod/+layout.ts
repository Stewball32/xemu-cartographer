import { requireAdmin } from '$lib/utils/guards';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ url, parent }) => {
	await parent();
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
