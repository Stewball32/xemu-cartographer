import type { PageLoad } from './$types';

export const ssr = false;

export const load: PageLoad = async ({ parent }) => {
	await parent();
	return {};
};
