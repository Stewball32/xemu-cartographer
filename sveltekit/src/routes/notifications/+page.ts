import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { buildLoginUrl } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

// Notifications page — M23b.
//
// Redirect-on-anon mirrors /settings/'s pattern: catch unauth at load
// time so the page doesn't briefly render the empty state before the
// root layout's redirect kicks in. requiresAuth flag also belt-and-
// suspenders the root layout guard.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	return { requiresAuth: true };
};
