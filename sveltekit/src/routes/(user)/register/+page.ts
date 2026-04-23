import { safeRedirectTarget } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ url }) => {
	return { redirectTo: safeRedirectTarget(url.searchParams.get('redirect')) };
};
