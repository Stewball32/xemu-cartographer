import type { PageLoad } from './$types';

// Dynamic route — relies on the static adapter's index.html SPA fallback at
// runtime since there's no static list of names to crawl from /pod/. The
// parent /pod/+layout.ts handles requireAdmin, so this load only needs to
// expose params.name to the page.
export const prerender = false;

export const load: PageLoad = async ({ params, parent }) => {
	await parent();
	return { name: params.name };
};
