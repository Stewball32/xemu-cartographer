import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { buildLoginUrl } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

// /play/ is for logged-in players (not admin-only) — unauthenticated visitors
// go to login first, then back here.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	return { requiresAuth: true };
};
