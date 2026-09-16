import { requireAdmin } from '$lib/utils/guards';
import type { PageLoad } from './$types';

// /admin/tokens/ — opaque-key management (authz design §9). The admin layout
// already gates the group on isAdmin; the guard is repeated here so the page
// stays closed if it is ever moved out from under /admin/. The server is the
// real gate: every call needs token.list / token.mint:<kind> /
// token.revoke:<kid>.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	requireAdmin(url);
	return { requiresAuth: true, isAdmin: true };
};
