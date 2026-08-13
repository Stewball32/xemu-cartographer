import { requireOrganizer } from '$lib/utils/guards';
import type { LayoutLoad } from './$types';

// Gate the /organizer route group to organizers (or admins). The backend PB
// rules on gametypes / isos are the real enforcement; this keeps
// non-organizers from landing on a page they can't use.
export const load: LayoutLoad = async ({ url, parent }) => {
	await parent();
	requireOrganizer(url);
	return { requiresAuth: true, isOrganizer: true };
};
