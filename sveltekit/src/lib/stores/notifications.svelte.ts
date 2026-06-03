// notifications.svelte.ts — M23b
//
// Svelte 5 runes singleton that powers the header bell + /notifications/
// page. Reads through the PocketBase SDK against the `notifications`
// collection (M23a), so PB rules enforce ownership (recipient or admin)
// without an extra server-side endpoint.
//
// Polling: triggered on initial mount (called from the layout) and on the
// window `focus` event. There's no setInterval — that wins us the
// "poll-on-focus is fine for MVP" trade in the M23 doc without leaving a
// timer running when the tab is backgrounded. M23+ may upgrade to WS push.

import pb from '$lib/pocketbase';
import { auth } from '$lib/stores/auth.svelte';
import { describeAsyncError, toaster } from '$lib/stores/toaster';

export interface NotificationRow {
	id: string;
	user: string;
	type: string;
	payload_json: string;
	read: boolean;
	read_at: string | null;
	created: string;
}

const RECENT_LIMIT = 20;

function createNotificationsStore() {
	let unreadCount = $state(0);
	let recent = $state<NotificationRow[]>([]);
	let loading = $state(false);
	let inFlight: Promise<void> | null = null;
	let focusBound = false;

	async function refreshInternal() {
		if (!auth.isLoggedIn) {
			unreadCount = 0;
			recent = [];
			return;
		}
		loading = true;
		try {
			const list = await pb
				.collection('notifications')
				.getList<NotificationRow>(1, RECENT_LIMIT, { sort: '-created' });
			recent = list.items;
			// Bell count walks the unread index server-side; the SDK exposes
			// it via getList with a filter, but the dedicated server-side
			// count avoids paginating just to count.
			const unread = await pb.collection('notifications').getList<NotificationRow>(1, 1, {
				filter: 'read = false',
				fields: 'id'
			});
			unreadCount = unread.totalItems;
		} catch (err) {
			// Silent on the store side — the bell UI still shows the last
			// known good count rather than zeroing out on transient errors.
			console.warn('notifications.refresh failed:', describeAsyncError(err));
		} finally {
			loading = false;
		}
	}

	function ensureFocusHandler() {
		if (focusBound || typeof window === 'undefined') return;
		window.addEventListener('focus', () => void refresh());
		focusBound = true;
	}

	function refresh(): Promise<void> {
		if (inFlight) return inFlight;
		ensureFocusHandler();
		inFlight = refreshInternal().finally(() => {
			inFlight = null;
		});
		return inFlight;
	}

	async function markRead(id: string) {
		try {
			await pb.collection('notifications').update(id, { read: true });
			recent = recent.map((r) => (r.id === id ? { ...r, read: true } : r));
			if (unreadCount > 0) unreadCount -= 1;
		} catch (err) {
			toaster.error({
				title: 'Mark read failed',
				description: describeAsyncError(err)
			});
		}
	}

	async function markAllRead() {
		const unreadRows = recent.filter((r) => !r.read);
		if (unreadRows.length === 0) {
			await refresh();
			return;
		}
		try {
			await Promise.all(
				unreadRows.map((r) => pb.collection('notifications').update(r.id, { read: true }))
			);
			recent = recent.map((r) => ({ ...r, read: true }));
			// Re-read from the server so any rows we didn't have on the page
			// stay accurately counted (a user with >20 unread will still
			// have some left after a UI mark-all).
			await refresh();
		} catch (err) {
			toaster.error({
				title: 'Mark all read failed',
				description: describeAsyncError(err)
			});
		}
	}

	function clear() {
		unreadCount = 0;
		recent = [];
	}

	// Logout / token rotation cascade. We subscribe to pb.authStore directly
	// rather than going through the auth.svelte.ts store to keep the import
	// graph one-way (auth → notifications would close a circular path).
	pb.authStore.onChange((newToken) => {
		if (!newToken) {
			clear();
			return;
		}
		void refresh();
	});

	return {
		get unreadCount() {
			return unreadCount;
		},
		get recent() {
			return recent;
		},
		get loading() {
			return loading;
		},
		refresh,
		markRead,
		markAllRead,
		clear
	};
}

export const notifications = createNotificationsStore();
