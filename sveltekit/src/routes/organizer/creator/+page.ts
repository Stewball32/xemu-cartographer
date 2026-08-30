import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// The creator workspace was absorbed into /organizer/gametypes/ (library +
// editor, master-detail). Keep this path working for existing links.
export const load: PageLoad = async () => {
	throw redirect(303, '/organizer/gametypes/');
};
