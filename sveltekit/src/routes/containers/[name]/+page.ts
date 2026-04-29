import { auth } from '$lib/stores/auth.svelte';
import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

// Dynamic route — relies on the static adapter's index.html SPA fallback at
// runtime since there's no static list of names to crawl from /containers/.
export const prerender = false;

export const load: PageLoad = async ({ url }) => {
	await auth.ready;
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
