import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { auth } from '$lib/stores/auth.svelte';
import { buildLoginUrl } from '$lib/utils/redirect';

// Used in +page.ts load() functions to gate admin-only routes.
// Mirrors backend middleware.RequireAdmin: superusers and users.isAdmin=true
// pass; everyone else is redirected to /. Unauthenticated users hit the login
// page first. The root +layout.ts load awaits auth.hydrate(), so by the time
// any child page load calls this guard, isAdmin is settled.
export function requireAdmin(url: URL): void {
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	if (!auth.isAdmin) {
		throw redirect(303, '/');
	}
}

export function isAdmin(): boolean {
	return auth.isAdmin;
}
