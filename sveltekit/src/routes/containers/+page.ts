import { auth } from '$lib/stores/auth.svelte';
import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url }) => {
	await auth.ready;
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
