import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// /organizer/ has no page of its own — send organizers to the creator workspace.
export const load: PageLoad = async () => {
	throw redirect(303, '/organizer/creator/');
};
