import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = async ({ url, params }) => {
	requireAdmin(url);
	return { name: params.name };
};
