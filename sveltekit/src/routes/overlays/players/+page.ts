import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { buildLoginUrl } from '$lib/utils/redirect';
import type { PageLoad } from './$types';

// The overlay WebSocket room requires authentication (any role); admin not
// required. Late-joiner replay arrives as a "current_state" envelope built
// from the runner's instanceCache — see internal/websocket/handlers/join_room.go.
export const load: PageLoad = async ({ url, parent }) => {
	await parent();
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	return { requiresAuth: true };
};
