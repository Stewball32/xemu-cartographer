import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

// await parent() so +layout.ts's auth.hydrate() has settled isAdmin before
// requireAdmin reads it — SvelteKit runs parent layout and child page loads
// in parallel by default.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
