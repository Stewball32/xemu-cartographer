import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// /organizer/ has no page of its own — land on the disc library (the daily
// surface; the other organizer pages are one rail click away).
export const load: PageLoad = async () => {
	throw redirect(303, '/organizer/discs/');
};
