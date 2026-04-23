import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { buildLoginUrl } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url }) => {
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	return { requiresAuth: true };
};
