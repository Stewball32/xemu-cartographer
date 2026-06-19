import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { buildLoginUrl } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

// RequireAuth (not admin) — the page itself shows an "unauthorized" state for
// signed-in users without the manage-overlays permission; the backend is the
// real gate.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	return { requiresAuth: true };
};
