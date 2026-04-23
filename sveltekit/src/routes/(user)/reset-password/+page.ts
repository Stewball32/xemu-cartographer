import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url }) => {
	if (pb.authStore.isValid) {
		throw redirect(303, '/');
	}
	return { token: url.searchParams.get('token') ?? '' };
};
