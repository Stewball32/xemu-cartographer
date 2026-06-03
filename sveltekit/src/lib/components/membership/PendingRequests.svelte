<script lang="ts">
	// PendingRequests — M23d
	//
	// Two-bucket list of the caller's pending team_membership_requests:
	//   - Incoming: invites where direction=invited and user=me (Accept / Decline)
	//   - Outgoing: rows where initiated_by=me (Cancel)
	// Renders inline above the "Teams you are on" list in /settings/ so the
	// user has a one-stop view of pending state without leaving the page.
	//
	// Hits the PocketBase SDK directly with the rules from
	// schema/team_membership_requests.go (list/view = user OR initiated_by OR
	// admin). One read per mount + refresh after each action.

	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { ClockIcon, CheckIcon, XIcon } from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import { acceptRequest, cancelRequest, declineRequest } from '$lib/utils/membership-actions';
	import { notifications } from '$lib/stores/notifications.svelte';

	interface TeamRow {
		id: string;
		name: string;
		slug: string;
	}
	interface UserRow {
		id: string;
		username: string;
		name?: string;
	}
	interface MembershipRequestRow {
		id: string;
		team: string;
		user: string;
		direction: 'invited' | 'requested';
		status: 'pending' | 'accepted' | 'declined' | 'expired' | 'cancelled';
		initiated_by: string;
		created: string;
		expand?: { team?: TeamRow; user?: UserRow; initiated_by?: UserRow };
	}

	let incoming = $state<MembershipRequestRow[]>([]);
	let outgoing = $state<MembershipRequestRow[]>([]);
	let loading = $state(true);
	let actionBusy = $state<Record<string, boolean>>({});

	async function load() {
		loading = true;
		if (!auth.user) {
			loading = false;
			return;
		}
		const me = auth.user.id;
		try {
			const list = await pb
				.collection('team_membership_requests')
				.getFullList<MembershipRequestRow>({
					filter: `status = "pending" && (user = "${me}" || initiated_by = "${me}")`,
					expand: 'team,user,initiated_by',
					sort: '-created'
				});
			incoming = list.filter((r) => r.user === me && r.direction === 'invited');
			outgoing = list.filter((r) => r.initiated_by === me);
		} catch (err) {
			toaster.error({
				title: 'Pending requests load failed',
				description: describeAsyncError(err)
			});
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
	});

	async function doAccept(row: MembershipRequestRow) {
		actionBusy = { ...actionBusy, [row.id]: true };
		try {
			await toastPromise(acceptRequest(row.id), {
				loading: { title: 'Accepting', description: row.expand?.team?.name },
				success: { title: 'Accepted', description: row.expand?.team?.name },
				errorTitle: 'Accept failed'
			});
			await Promise.all([load(), notifications.refresh()]);
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[row.id];
			actionBusy = next;
		}
	}

	async function doDecline(row: MembershipRequestRow) {
		actionBusy = { ...actionBusy, [row.id]: true };
		try {
			await toastPromise(declineRequest(row.id), {
				loading: { title: 'Declining', description: row.expand?.team?.name },
				success: { title: 'Declined', description: row.expand?.team?.name },
				errorTitle: 'Decline failed'
			});
			await Promise.all([load(), notifications.refresh()]);
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[row.id];
			actionBusy = next;
		}
	}

	async function doCancel(row: MembershipRequestRow) {
		actionBusy = { ...actionBusy, [row.id]: true };
		try {
			await toastPromise(cancelRequest(row.id), {
				loading: { title: 'Cancelling', description: row.expand?.team?.name },
				success: { title: 'Cancelled', description: row.expand?.team?.name },
				errorTitle: 'Cancel failed'
			});
			await Promise.all([load(), notifications.refresh()]);
		} catch {
			// toast already shown
		} finally {
			const next = { ...actionBusy };
			delete next[row.id];
			actionBusy = next;
		}
	}

	function formatTime(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		return d.toLocaleString();
	}

	const hasAny = $derived(incoming.length > 0 || outgoing.length > 0);
</script>

{#if loading}
	<!-- Stay quiet — the parent section already has its own list rendering. -->
{:else if hasAny}
	<div class="space-y-2">
		{#if incoming.length > 0}
			<h3 class="text-xs font-semibold tracking-wide uppercase opacity-60">Invitations</h3>
			<ul class="space-y-2">
				{#each incoming as row (row.id)}
					{@const team = row.expand?.team}
					<li
						class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-surface-300-700 p-3"
					>
						<div class="min-w-0 flex-1">
							<p class="text-sm font-medium">
								{#if team}
									Invited to
									<a href={resolve('/teams/[slug]', { slug: team.slug })} class="anchor"
										>{team.name}</a
									>
								{:else}
									Invite to [unknown team]
								{/if}
							</p>
							<p class="text-[11px] opacity-60">
								<ClockIcon class="inline-block size-3" />
								{formatTime(row.created)}
							</p>
						</div>
						<div class="flex shrink-0 items-center gap-1">
							<button
								type="button"
								class="btn preset-tonal-error btn-sm"
								disabled={actionBusy[row.id]}
								onclick={() => doDecline(row)}
							>
								<XIcon class="size-3.5" />
								Decline
							</button>
							<button
								type="button"
								class="btn preset-filled-primary-500 btn-sm"
								disabled={actionBusy[row.id]}
								onclick={() => doAccept(row)}
							>
								<CheckIcon class="size-3.5" />
								Accept
							</button>
						</div>
					</li>
				{/each}
			</ul>
		{/if}

		{#if outgoing.length > 0}
			<h3 class="mt-3 text-xs font-semibold tracking-wide uppercase opacity-60">
				Sent (awaiting response)
			</h3>
			<ul class="space-y-2">
				{#each outgoing as row (row.id)}
					{@const team = row.expand?.team}
					{@const recipient = row.expand?.user}
					<li
						class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-surface-300-700 p-3"
					>
						<div class="min-w-0 flex-1">
							<p class="text-sm font-medium">
								{#if row.direction === 'invited' && recipient && team}
									Invited
									<a
										href={resolve('/u/[username]', { username: recipient.username })}
										class="anchor">@{recipient.username}</a
									>
									to
									<a href={resolve('/teams/[slug]', { slug: team.slug })} class="anchor"
										>{team.name}</a
									>
								{:else if team}
									Requested to join
									<a href={resolve('/teams/[slug]', { slug: team.slug })} class="anchor"
										>{team.name}</a
									>
								{:else}
									Pending request
								{/if}
							</p>
							<p class="text-[11px] opacity-60">
								<ClockIcon class="inline-block size-3" />
								{formatTime(row.created)}
							</p>
						</div>
						<button
							type="button"
							class="btn preset-tonal btn-sm"
							disabled={actionBusy[row.id]}
							onclick={() => doCancel(row)}
						>
							<XIcon class="size-3.5" />
							Cancel
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/if}
