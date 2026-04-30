import { auth } from '$lib/stores/auth.svelte';
import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = async ({ url, params }) => {
	await auth.ready;
	requireAdmin(url);
	return { name: params.name };
};
