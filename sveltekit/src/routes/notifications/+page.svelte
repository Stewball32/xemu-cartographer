<script lang="ts">
	// Notifications page — M23b
	//
	// Full-page list, complementing the header bell's recent-N dropdown.
	// Groups rows by Today / This week / Older for skimmability; per-row
	// click marks read + navigates; the page-level Mark all read flushes
	// the inbox. Pagination via Skeleton primitives is the next thing to
	// add when the list outgrows the initial 50.

	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import {
		ArrowLeftIcon,
		CheckCheckIcon,
		CheckIcon,
		MailIcon,
		RefreshCwIcon,
		XIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { notifications, type NotificationRow } from '$lib/stores/notifications.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import {
		extractRequestID,
		isActionableInvite,
		renderNotification
	} from '$lib/utils/notification-render';
	import { acceptRequest, declineRequest } from '$lib/utils/membership-actions';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';

	const PAGE_SIZE = 50;

	let rows = $state<NotificationRow[]>([]);
	let loading = $state(false);
	let loadError = $state<string | null>(null);

	async function load() {
		loading = true;
		loadError = null;
		try {
			const list = await pb
				.collection('notifications')
				.getList<NotificationRow>(1, PAGE_SIZE, { sort: '-created' });
			rows = list.items;
		} catch (err) {
			loadError = describeAsyncError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
		// Keep the bell store in sync — focus may not have fired since the
		// user opened the page via the dropdown's See-all link.
		void notifications.refresh();
	});

	async function markRead(row: NotificationRow) {
		if (row.read) return;
		try {
			await pb.collection('notifications').update(row.id, { read: true });
			rows = rows.map((r) => (r.id === row.id ? { ...r, read: true } : r));
			await notifications.refresh();
		} catch (err) {
			toaster.error({ title: 'Mark read failed', description: describeAsyncError(err) });
		}
	}

	async function handleRowClick(row: NotificationRow, href: string | null) {
		await markRead(row);
		if (href) {
			// resolve() values are runtime-validated, not static literals.
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			await goto(href);
		}
	}

	async function markAllRead() {
		const unreadRows = rows.filter((r) => !r.read);
		if (unreadRows.length === 0) return;
		try {
			await Promise.all(
				unreadRows.map((r) => pb.collection('notifications').update(r.id, { read: true }))
			);
			rows = rows.map((r) => ({ ...r, read: true }));
			await notifications.refresh();
		} catch (err) {
			toaster.error({ title: 'Mark all read failed', description: describeAsyncError(err) });
		}
	}

	function dayBucket(iso: string): 'today' | 'week' | 'older' {
		const t = new Date(iso).getTime();
		if (Number.isNaN(t)) return 'older';
		const now = Date.now();
		const dayMs = 24 * 60 * 60 * 1000;
		if (now - t < dayMs) return 'today';
		if (now - t < 7 * dayMs) return 'week';
		return 'older';
	}

	const grouped = $derived.by(() => {
		const buckets: Record<'today' | 'week' | 'older', NotificationRow[]> = {
			today: [],
			week: [],
			older: []
		};
		for (const r of rows) buckets[dayBucket(r.created)].push(r);
		return buckets;
	});

	function formatTime(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		return d.toLocaleString();
	}

	const unreadOnPage = $derived(rows.filter((r) => !r.read).length);

	let actionBusy = $state<Record<string, boolean>>({});

	async function handleAccept(row: NotificationRow, requestID: string) {
		actionBusy = { ...actionBusy, [row.id]: true };
		try {
			await toastPromise(acceptRequest(requestID), {
				loading: { title: 'Accepting' },
				success: { title: 'Accepted' },
				errorTitle: 'Accept failed'
			});
			await markRead(row);
			await load();
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[row.id];
			actionBusy = next;
		}
	}

	async function handleDecline(row: NotificationRow, requestID: string) {
		actionBusy = { ...actionBusy, [row.id]: true };
		try {
			await toastPromise(declineRequest(requestID), {
				loading: { title: 'Declining' },
				success: { title: 'Declined' },
				errorTitle: 'Decline failed'
			});
			await markRead(row);
			await load();
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[row.id];
			actionBusy = next;
		}
	}
</script>

<div class="mx-auto flex max-w-3xl flex-col gap-4">
	<a href={resolve('/')} class="inline-flex items-center gap-1 anchor text-sm">
		<ArrowLeftIcon class="size-4" />
		Back home
	</a>

	<PageHeader title="Notifications" description="Recent activity across your gamertags and teams.">
		{#snippet actions()}
			<button
				type="button"
				class="btn preset-tonal btn-sm"
				onclick={load}
				disabled={loading}
				title="Refresh"
			>
				<RefreshCwIcon class="size-4 {loading ? 'animate-spin' : ''}" />
				<span>Refresh</span>
			</button>
			<button
				type="button"
				class="btn preset-filled-primary-500 btn-sm"
				onclick={markAllRead}
				disabled={unreadOnPage === 0}
				title={unreadOnPage === 0
					? 'No unread notifications'
					: 'Mark every visible notification as read'}
			>
				<CheckCheckIcon class="size-4" />
				<span>Mark all read</span>
			</button>
		{/snippet}
	</PageHeader>

	{#if loadError}
		<Card class="preset-tonal-error!">
			<p class="text-sm">Failed to load notifications: {loadError}</p>
		</Card>
	{/if}

	{#if rows.length === 0 && !loading}
		<Card>
			<div class="px-2 py-8 text-center text-sm opacity-70">
				<MailIcon class="mx-auto mb-2 size-8 opacity-50" />
				<p>No notifications yet. Invitations and join requests will show up here.</p>
			</div>
		</Card>
	{:else}
		{#each ['today', 'week', 'older'] as const as bucket (bucket)}
			{@const list = grouped[bucket]}
			{#if list.length > 0}
				<section class="flex flex-col gap-1">
					<h2 class="px-2 text-xs font-semibold tracking-wide uppercase opacity-60">
						{bucket === 'today' ? 'Today' : bucket === 'week' ? 'This week' : 'Older'}
					</h2>
					<Card size="flush">
						<ul class="divide-y divide-surface-200-800">
							{#each list as n (n.id)}
								{@const r = renderNotification(n.type, n.payload_json)}
								{@const requestID = extractRequestID(n.type, n.payload_json)}
								{@const showActions = isActionableInvite(n.type) && requestID && !n.read}
								<li class:opacity-60={n.read}>
									<button
										type="button"
										class="block w-full px-4 py-3 text-left hover:preset-tonal"
										onclick={() => handleRowClick(n, r.href)}
									>
										<div class="flex items-start justify-between gap-3">
											<div class="min-w-0 flex-1">
												<p class="text-sm font-medium">{r.title}</p>
												<p class="mt-0.5 text-xs opacity-75">{r.description}</p>
												<p class="mt-1 text-[10px] opacity-50">{formatTime(n.created)}</p>
											</div>
											{#if !n.read}
												<span
													class="mt-1.5 inline-block size-2.5 shrink-0 rounded-full bg-primary-500"
													aria-label="Unread"
												></span>
											{/if}
										</div>
									</button>
									{#if showActions}
										<div class="flex items-center justify-end gap-1 px-4 pb-2">
											<button
												type="button"
												class="btn preset-tonal-error btn-sm"
												disabled={actionBusy[n.id]}
												onclick={() => handleDecline(n, requestID!)}
											>
												<XIcon class="size-3.5" />
												Decline
											</button>
											<button
												type="button"
												class="btn preset-filled-primary-500 btn-sm"
												disabled={actionBusy[n.id]}
												onclick={() => handleAccept(n, requestID!)}
											>
												<CheckIcon class="size-3.5" />
												Accept
											</button>
										</div>
									{/if}
								</li>
							{/each}
						</ul>
					</Card>
				</section>
			{/if}
		{/each}
	{/if}
</div>
