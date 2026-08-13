import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// The gametype library + editor moved into the unified /organizer/creator/
// workspace. Keep this path working for existing links / muscle memory.
export const load: PageLoad = async () => {
	throw redirect(303, '/organizer/creator/');
};
