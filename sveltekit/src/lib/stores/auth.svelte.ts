import pb from '$lib/pocketbase';
import { apiBaseURL } from '$lib/utils/api-base';
import { hasScope as matchAnyScope } from '$lib/utils/scopes';
import type { UsersResponse } from '$lib/types/pocketbase-types';

const baseURL = apiBaseURL();

// M08: /api/me returns `roles: string[]` (the slugs the caller holds via
// user_roles). `isAdmin` is kept as a derived shorthand for FE backwards-
// compat; new consumers should branch on `roles` so future M16-style gates
// (`tournament_organizer`, `content_moderator`) work without another bump.
//
// authz (design §7.1 R-15 / §9): `scopes` is the union of the caller's role
// scopes (canonical, sorted — the same list the server's authz.Can matches
// against), `level` the max role level, `principal_kind` the resolver's kind
// ("pb_user" for a users JWT, "superuser" for _superusers). Superusers report
// scopes ["*"] and level 1000. Consumers that only need to *hide* UI should
// prefer `auth.hasScope(want)` over role checks — it mirrors the server's
// decision exactly, so a hidden button is one the server would reject anyway.
interface MeResponse {
	isAdmin: boolean;
	isSuperuser: boolean;
	roles: string[];
	scopes: string[];
	principal_kind: string;
	level: number;
}

interface Identity {
	roles: string[];
	scopes: string[];
	principalKind: string;
	level: number;
	isSuperuser: boolean;
}

const anonymous: Identity = {
	roles: [],
	scopes: [],
	principalKind: '',
	level: 0,
	isSuperuser: false
};

function createAuthStore() {
	let user = $state<UsersResponse | null>(pb.authStore.record as UsersResponse | null);
	let token = $state(pb.authStore.token);
	let roles = $state<string[]>([]);
	let scopes = $state<string[]>([]);
	let principalKind = $state('');
	let level = $state(0);
	let isSuperuser = $state(false);
	const isAdmin = $derived(isSuperuser || roles.includes('admin'));
	const isLoggedIn = $derived(token !== '' && user !== null);

	let hydratePromise: Promise<void> | null = null;

	function apply(id: Identity) {
		roles = id.roles;
		scopes = id.scopes;
		principalKind = id.principalKind;
		level = id.level;
		isSuperuser = id.isSuperuser;
	}

	// fetchIdentity resolves /api/me into the store's identity slice. Any
	// failure (network, 401 for a banned/deleted account, malformed body)
	// collapses to the anonymous identity — no roles, no scopes — so a
	// broken probe fails closed rather than leaving stale grants in place.
	async function fetchIdentity(authToken: string): Promise<Identity> {
		try {
			const res = await fetch(`${baseURL}/api/me`, {
				headers: { Authorization: authToken }
			});
			if (!res.ok) return anonymous;
			const data = (await res.json()) as MeResponse;
			return {
				roles: Array.isArray(data.roles) ? data.roles : [],
				scopes: Array.isArray(data.scopes)
					? data.scopes.filter((s): s is string => typeof s === 'string')
					: [],
				principalKind: typeof data.principal_kind === 'string' ? data.principal_kind : '',
				level: typeof data.level === 'number' && Number.isFinite(data.level) ? data.level : 0,
				isSuperuser: data.isSuperuser === true
			};
		} catch {
			return anonymous;
		}
	}

	async function refreshRoles() {
		const authToken = pb.authStore.token;
		if (!authToken) {
			apply(anonymous);
			return;
		}
		const result = await fetchIdentity(authToken);
		// Drop the result if the token rotated while we were in flight.
		if (pb.authStore.token === authToken) {
			apply(result);
		}
	}

	pb.authStore.onChange((newToken, record) => {
		user = (record as UsersResponse | null) ?? null;
		token = newToken;
		void refreshRoles();
	});

	// Single-shot initial hydration. The root +layout.ts load awaits this
	// before any child route's load runs, so guards see a settled isAdmin
	// (derived from roles) instead of racing the onChange listener.
	// Subsequent login/logout flows update via the onChange handler above.
	function hydrate(): Promise<void> {
		if (hydratePromise) return hydratePromise;
		hydratePromise = (async () => {
			if (pb.authStore.isValid) {
				// authStore.isValid is local-only (JWT expiry check). Probe the
				// server to catch tokens signed by a now-gone secret — e.g.
				// when `task dev` wipes tmp/pb_data/.
				try {
					await pb.collection('users').authRefresh();
				} catch {
					pb.authStore.clear();
				}
			}
			const authToken = pb.authStore.token;
			if (!authToken) {
				apply(anonymous);
				return;
			}
			apply(await fetchIdentity(authToken));
		})();
		return hydratePromise;
	}

	return {
		get user() {
			return user;
		},
		get token() {
			return token;
		},
		get isLoggedIn() {
			return isLoggedIn;
		},
		get roles() {
			return roles;
		},
		get scopes() {
			return scopes;
		},
		get principalKind() {
			return principalKind;
		},
		get level() {
			return level;
		},
		get isSuperuser() {
			return isSuperuser;
		},
		get isAdmin() {
			return isAdmin;
		},
		hasRole(slug: string): boolean {
			return roles.includes(slug);
		},
		/**
		 * hasScope reports whether the caller's scopes grant `want`
		 * ("<action>" or "<action>:<selector>") using the same matcher the
		 * server runs. Anonymous callers hold no scopes and always get false.
		 */
		hasScope(want: string): boolean {
			return matchAnyScope(scopes, want);
		},
		hydrate,
		// username is a required, immutable users field (min 2 / max 34; see
		// migrations snapshot + hooks/users_username_immutable.go) — the create
		// 400s with validation_required without it.
		async register(username: string, email: string, password: string, passwordConfirm: string) {
			await pb.collection('users').create({ username, email, password, passwordConfirm });
			await pb.collection('users').authWithPassword(email, password);
		},
		async login(email: string, password: string) {
			await pb.collection('users').authWithPassword(email, password);
		},
		async loginWithOAuth(provider: string) {
			await pb.collection('users').authWithOAuth2({ provider });
		},
		async listExternalAuths(userId: string) {
			return await pb.collection('users').listExternalAuths(userId);
		},
		async linkOAuth(provider: string) {
			await pb.collection('users').authWithOAuth2({ provider });
		},
		async unlinkOAuth(userId: string, provider: string) {
			await pb.collection('users').unlinkExternalAuth(userId, provider);
		},
		logout() {
			pb.authStore.clear();
		},
		async requestPasswordReset(email: string) {
			await pb.collection('users').requestPasswordReset(email);
		},
		async confirmPasswordReset(token: string, password: string, passwordConfirm: string) {
			await pb.collection('users').confirmPasswordReset(token, password, passwordConfirm);
		},
		async requestVerification(email: string) {
			await pb.collection('users').requestVerification(email);
		},
		async confirmVerification(token: string) {
			await pb.collection('users').confirmVerification(token);
		}
	};
}

export const auth = createAuthStore();
