import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// "Games" was renamed to "Discs" (the page manages disc images, not game
// entries). Keep the old path working for links / muscle memory.
export const load: PageLoad = async () => {
	throw redirect(303, '/organizer/discs/');
};
