// Playwright auth helper. Seeds localStorage with a fake admin auth record so
// requireAdmin() passes without a backend, and intercepts the two endpoints
// auth.svelte.ts hits during hydrate(): users/auth-refresh and /api/me.
//
// Used together with installPodMocks() from ./mocks — together they let a spec
// screenshot any /pod/* route against the static sirv build without booting
// PocketBase. Workers should not modify this file.

import type { Page } from '@playwright/test';

// Fake JWT with a far-future expiry. PocketBase's BaseAuthStore.isValid only
// decodes the payload to check `exp`; it doesn't verify the signature.
// Header / payload encoded as base64url (no padding, - and _ instead of + and /).
const FAKE_TOKEN = (() => {
	const header = { alg: 'HS256', typ: 'JWT' };
	const payload = {
		id: 'test-admin',
		type: 'auth',
		collectionId: '_pb_users_auth_',
		exp: 9999999999 // 2286-11-20
	};
	const b64url = (obj: unknown) =>
		Buffer.from(JSON.stringify(obj))
			.toString('base64')
			.replace(/\+/g, '-')
			.replace(/\//g, '_')
			.replace(/=+$/, '');
	return `${b64url(header)}.${b64url(payload)}.fakesig`;
})();

const ADMIN_RECORD = {
	id: 'test-admin',
	collectionId: '_pb_users_auth_',
	collectionName: 'users',
	email: 'admin@dev.com',
	emailVisibility: true,
	verified: true,
	username: 'admin',
	isAdmin: true,
	name: 'Test Admin',
	created: '2026-01-01T00:00:00.000Z',
	updated: '2026-01-01T00:00:00.000Z'
};

const AUTH_REFRESH_BODY = JSON.stringify({
	token: FAKE_TOKEN,
	record: ADMIN_RECORD
});

const ME_BODY = JSON.stringify({
	isAdmin: true,
	isSuperuser: false
});

export async function loginAsAdmin(page: Page): Promise<void> {
	// Seed PocketBase JS SDK's localStorage key before any app script runs.
	// SDK serializes { token, record } as JSON under `pocketbase_auth`.
	await page.addInitScript(
		([token, record]) => {
			localStorage.setItem('pocketbase_auth', JSON.stringify({ token, record }));
		},
		[FAKE_TOKEN, ADMIN_RECORD]
	);

	// auth.svelte.ts hydrate() probes the server to catch stale tokens.
	// Return success so isValid stays true.
	await page.route('**/api/collections/users/auth-refresh', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: AUTH_REFRESH_BODY
		})
	);

	// auth.svelte.ts fetchAdmin() checks isAdmin via the custom /api/me route.
	await page.route('**/api/me', (route) =>
		route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: ME_BODY
		})
	);
}
