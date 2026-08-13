<script lang="ts">
	// HeaderBell — M23b
	//
	// Bell icon + unread-count badge in the AppBar trail, paired with a
	// Skeleton Popover dropdown that lists the most-recent 20 rows. Renders
	// the per-type title/description/link via the shared
	// notification-render helper so the bell stays free of per-type render
	// logic. Mark-as-read on click; Mark-all-read clears the badge.

	import { Popover } from '@skeletonlabs/skeleton-svelte';
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { BellIcon, CheckCheckIcon, CheckIcon, MailIcon, XIcon } from '@lucide/svelte';
	import { notifications } from '$lib/stores/notifications.svelte';
	import {
		extractRequestID,
		isActionableInvite,
		renderNotification
	} from '$lib/utils/notification-render';
	import { acceptRequest, declineRequest } from '$lib/utils/membership-actions';
	import { toastPromise } from '$lib/stores/toaster';

	let open = $state(false);

	onMount(() => {
		void notifications.refresh();
	});

	function formatRelative(iso: string): string {
		const t = new Date(iso).getTime();
		if (Number.isNaN(t)) return '';
		const diff = Date.now() - t;
		const sec = Math.floor(diff / 1000);
		if (sec < 60) return `${sec}s ago`;
		const min = Math.floor(sec / 60);
		if (min < 60) return `${min}m ago`;
		const hr = Math.floor(min / 60);
		if (hr < 24) return `${hr}h ago`;
		const day = Math.floor(hr / 24);
		if (day < 7) return `${day}d ago`;
		return new Date(iso).toLocaleDateString();
	}

	async function handleRowClick(id: string, href: string | null, read: boolean) {
		if (!read) {
			await notifications.markRead(id);
		}
		open = false;
		if (href) {
			// resolve() values are runtime-validated; the linter wants static literals.
			// eslint-disable-next-line svelte/no-navigation-without-resolve
			await goto(href);
		}
	}

	async function handleMarkAllRead() {
		await notifications.markAllRead();
	}

	let actionBusy = $state<Record<string, boolean>>({});

	async function handleAccept(id: string, requestID: string) {
		actionBusy = { ...actionBusy, [id]: true };
		try {
			await toastPromise(acceptRequest(requestID), {
				loading: { title: 'Accepting' },
				success: { title: 'Accepted' },
				errorTitle: 'Accept failed'
			});
			await notifications.markRead(id);
			await notifications.refresh();
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[id];
			actionBusy = next;
		}
	}

	async function handleDecline(id: string, requestID: string) {
		actionBusy = { ...actionBusy, [id]: true };
		try {
			await toastPromise(declineRequest(requestID), {
				loading: { title: 'Declining' },
				success: { title: 'Declined' },
				errorTitle: 'Decline failed'
			});
			await notifications.markRead(id);
			await notifications.refresh();
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[id];
			actionBusy = next;
		}
	}

	const badgeLabel = $derived.by(() => {
		const n = notifications.unreadCount;
		if (n <= 0) return '';
		if (n > 99) return '99+';
		return String(n);
	});
</script>

<Popover {open} onOpenChange={(d) => (open = d.open)} positioning={{ placement: 'bottom-end' }}>
	<Popover.Trigger
		class="relative flex size-9 items-center justify-center rounded-full hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-2"
		aria-label="Notifications"
		title="Notifications"
	>
		<BellIcon class="size-5" />
		{#if badgeLabel}
			<span
				class="absolute -top-0.5 -right-0.5 inline-flex min-w-4 items-center justify-center rounded-full bg-error-500 px-1 text-[10px] leading-none font-bold text-white"
				aria-label="{notifications.unreadCount} unread"
			>
				{badgeLabel}
			</span>
		{/if}
	</Popover.Trigger>
	<Popover.Positioner class="z-50!">
		<Popover.Content
			class="max-h-[28rem] w-80 overflow-hidden card preset-filled-surface-100-900 p-0 shadow-xl"
		>
			<div class="flex items-center justify-between border-b border-surface-200-800 px-3 py-2">
				<p class="text-sm font-semibold">Notifications</p>
				<button
					type="button"
					class="btn preset-tonal text-xs btn-sm"
					onclick={handleMarkAllRead}
					disabled={notifications.unreadCount === 0}
					title="Mark all as read"
				>
					<CheckCheckIcon class="size-3.5" />
					<span>Mark all read</span>
				</button>
			</div>

			<div class="max-h-72 overflow-y-auto">
				{#if notifications.recent.length === 0}
					<div class="px-4 py-8 text-center text-sm opacity-60">
						<MailIcon class="mx-auto mb-2 size-6 opacity-50" />
						<p>No notifications yet.</p>
					</div>
				{:else}
					<ul class="divide-y divide-surface-200-800">
						{#each notifications.recent as n (n.id)}
							{@const r = renderNotification(n.type, n.payload_json)}
							{@const requestID = extractRequestID(n.type, n.payload_json)}
							{@const showActions = isActionableInvite(n.type) && requestID && !n.read}
							<li class:opacity-60={n.read}>
								<button
									type="button"
									class="block w-full px-3 py-2 text-left hover:preset-tonal"
									onclick={() => handleRowClick(n.id, r.href, n.read)}
								>
									<div class="flex items-start justify-between gap-2">
										<p class="flex-1 text-sm font-medium">{r.title}</p>
										{#if !n.read}
											<span
												class="mt-1.5 inline-block size-2 shrink-0 rounded-full bg-primary-500"
												aria-label="Unread"
											></span>
										{/if}
									</div>
									<p class="mt-0.5 text-xs opacity-75">{r.description}</p>
									<p class="mt-0.5 text-[10px] opacity-50">{formatRelative(n.created)}</p>
								</button>
								{#if showActions}
									<div class="flex items-center justify-end gap-1 px-3 pb-2">
										<button
											type="button"
											class="btn preset-tonal-error text-xs btn-sm"
											disabled={actionBusy[n.id]}
											onclick={() => handleDecline(n.id, requestID!)}
										>
											<XIcon class="size-3.5" />
											Decline
										</button>
										<button
											type="button"
											class="btn preset-filled-primary-500 text-xs btn-sm"
											disabled={actionBusy[n.id]}
											onclick={() => handleAccept(n.id, requestID!)}
										>
											<CheckIcon class="size-3.5" />
											Accept
										</button>
									</div>
								{/if}
							</li>
						{/each}
					</ul>
				{/if}
			</div>

			<div class="border-t border-surface-200-800 px-3 py-2">
				<a
					href={resolve('/notifications/')}
					class="block text-center anchor text-xs"
					onclick={() => (open = false)}
				>
					See all notifications →
				</a>
			</div>
		</Popover.Content>
	</Popover.Positioner>
</Popover>
