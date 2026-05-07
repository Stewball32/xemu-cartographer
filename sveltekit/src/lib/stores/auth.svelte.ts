import pb from '$lib/pocketbase';
import { apiBaseURL } from '$lib/utils/api-base';
import type { UsersResponse } from '$lib/types/pocketbase-types';

const baseURL = apiBaseURL();

interface MeResponse {
	isAdmin: boolean;
	isSuperuser: boolean;
}

function createAuthStore() {
	let user = $state<UsersResponse | null>(pb.authStore.record as UsersResponse | null);
	let token = $state(pb.authStore.token);
	let isAdmin = $state(false);
	const isLoggedIn = $derived(token !== '' && user !== null);

	let hydratePromise: Promise<void> | null = null;

	async function fetchAdmin(authToken: string): Promise<boolean> {
		try {
			const res = await fetch(`${baseURL}/api/me`, {
				headers: { Authorization: authToken }
			});
			if (!res.ok) return false;
			const data = (await res.json()) as MeResponse;
			return data.isAdmin === true || data.isSuperuser === true;
		} catch {
			return false;
		}
	}

	async function refreshAdmin() {
		const authToken = pb.authStore.token;
		if (!authToken) {
			isAdmin = false;
			return;
		}
		const result = await fetchAdmin(authToken);
		// Drop the result if the token rotated while we were in flight.
		if (pb.authStore.token === authToken) isAdmin = result;
	}

	pb.authStore.onChange((newToken, record) => {
		user = (record as UsersResponse | null) ?? null;
		token = newToken;
		void refreshAdmin();
	});

	// Single-shot initial hydration. The root +layout.ts load awaits this
	// before any child route's load runs, so guards see a settled isAdmin
	// instead of racing the onChange listener. Subsequent login/logout flows
	// update isAdmin via the onChange handler above.
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
			isAdmin = authToken ? await fetchAdmin(authToken) : false;
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
		get isAdmin() {
			return isAdmin;
		},
		hydrate,
		async register(email: string, password: string, passwordConfirm: string) {
			await pb.collection('users').create({ email, password, passwordConfirm });
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
