import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// The gamertag identity page was consolidated into the tabbed /settings/ page
// (settings redesign) — its content lives on the Halo 2 / Halo: CE / Stream
// tabs now. Keep the old path working; land on the H2 tab.
export const load: PageLoad = async () => {
	throw redirect(303, '/settings/?tab=h2');
};
