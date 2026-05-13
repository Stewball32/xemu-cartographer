import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = async ({ params, parent }) => {
	await parent();
	return { name: params.name };
};
