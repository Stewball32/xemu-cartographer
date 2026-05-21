import { auth } from '$lib/stores/auth.svelte';
import type { LayoutLoad } from './$types';

export const ssr = false;
export const prerender = true;
export const trailingSlash = 'always';

// Hydrate auth before any child +page.ts load runs so route guards see a
// settled isAdmin instead of racing the pb.authStore.onChange listener.
// SvelteKit guarantees parent layout loads resolve before child loads.
export const load: LayoutLoad = async () => {
	await auth.hydrate();
	return {};
};
