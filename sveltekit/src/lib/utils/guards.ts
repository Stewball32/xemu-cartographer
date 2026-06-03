import { redirect } from '@sveltejs/kit';
import pb from '$lib/pocketbase';
import { auth } from '$lib/stores/auth.svelte';
import { buildLoginUrl } from '$lib/utils/redirect';

// Used in +page.ts load() functions to gate admin-only routes.
// Mirrors backend middleware.RequireAdmin: PB superusers + users holding a
// `user_roles` row pointing at the admin role pass; everyone else is
// redirected to /. Unauthenticated users hit the login page first. The root
// +layout.ts load awaits auth.hydrate(), so by the time any child page load
// calls this guard, the roles[] are settled.
export function requireAdmin(url: URL): void {
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	if (!auth.isAdmin) {
		throw redirect(303, '/');
	}
}

// requireRole gates a route on a specific role slug (e.g. "tournament_organizer"
// for future M16 pages). Superusers pass automatically. Mirrors the backend
// middleware.RequireRole that lands alongside the admin-users route group in
// 8g.
export function requireRole(url: URL, slug: string): void {
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	if (!auth.isSuperuser && !auth.hasRole(slug)) {
		throw redirect(303, '/');
	}
}

export function isAdmin(): boolean {
	return auth.isAdmin;
}
