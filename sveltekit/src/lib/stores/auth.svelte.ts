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

	let ready: Promise<void> = Promise.resolve();

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

	function refreshAdmin() {
		const authToken = pb.authStore.token;
		if (!authToken) {
			isAdmin = false;
			ready = Promise.resolve();
			return;
		}
		ready = fetchAdmin(authToken).then((result) => {
			// Drop the result if the token rotated while we were in flight.
			if (pb.authStore.token === authToken) isAdmin = result;
		});
	}

	pb.authStore.onChange((newToken, record) => {
		user = (record as UsersResponse | null) ?? null;
		token = newToken;
		refreshAdmin();
	});

	if (pb.authStore.isValid) {
		// authStore.isValid is local-only (JWT expiry check). Probe the server
		// to catch tokens signed by a now-gone secret — e.g. when `task dev`
		// wipes tmp/pb_data/. Cleared store fires onChange → state runes update.
		ready = pb
			.collection('users')
			.authRefresh()
			.then(() => undefined)
			.catch(() => {
				pb.authStore.clear();
			});
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
		get ready() {
			return ready;
		},
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
