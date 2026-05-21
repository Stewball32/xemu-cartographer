import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	await parent();
	if (pb.authStore.isValid) {
		throw redirect(303, '/');
	}
};
