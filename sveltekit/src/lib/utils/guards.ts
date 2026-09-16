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

// canManageLibrary reports whether the caller may curate the shared gametype
// library + game (XBE) uploads. authz (design §9): the primary check is the
// `library.manage` scope — the same want the server's authz.Can evaluates —
// so a role whose scopes were edited away stops rendering the organizer nav
// without a code change. The admin / `organizer` role fallback is kept for
// sessions hydrated from a pre-authz /api/me payload (no `scopes` field);
// the backend PB rules are the real gate either way.
export function canManageLibrary(): boolean {
	return auth.hasScope('library.manage') || auth.isAdmin || auth.hasRole('organizer');
}

// requireOrganizer gates the /organizer route group: organizers OR admins pass,
// everyone else is redirected (unauthenticated → login first). The backend PB
// rules are the real gate; this is just so the UI doesn't render a page the
// user can't use.
export function requireOrganizer(url: URL): void {
	if (!pb.authStore.isValid) {
		throw redirect(303, buildLoginUrl(url.pathname + url.search));
	}
	if (!canManageLibrary()) {
		throw redirect(303, '/');
	}
}
