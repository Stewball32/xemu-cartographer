import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	if (pb.authStore.isValid) {
		throw redirect(303, '/');
	}
};
