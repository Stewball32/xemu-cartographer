<script lang="ts">
	// Team page — M23c. Replaces the M6 placeholder.
	//
	// Public surface (any authed user can read; PB rules on teams + rosters
	// + team_log permit list/view to any authed caller). Shows:
	//   - header: name + slug + Founded by + status badge if non-approved
	//   - active roster with owner/manager badges
	//   - former members section (rosters with left_at set)
	//   - activity timeline reading team_log (member churn + renames)
	//   - "Request to join" CTA (visible when authed + not in roster + no
	//     pending request + caller has a usable default_gamertag)
	//
	// Per-member Remove + per-row Leave actions land in M23d; this slice
	// renders the data shape end-to-end so the schema is validated by use.

	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		ClockIcon,
		CrownIcon,
		BriefcaseIcon,
		HistoryIcon,
		LoaderIcon,
		LogInIcon,
		LogOutIcon,
		UserIcon,
		UserMinusIcon,
		UsersIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { confirmToast, describeAsyncError, toastPromise } from '$lib/stores/toaster';
	import {
		leaveRoster,
		removeRosterMember,
		requestToJoinTeam
	} from '$lib/utils/membership-actions';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { notifications } from '$lib/stores/notifications.svelte';

	const slug = $derived(page.params.slug ?? '');

	interface TeamRow {
		id: string;
		name: string;
		slug: string;
		status: string;
		created_by: string;
		created: string;
		expand?: { created_by?: UserRow };
	}
	interface UserRow {
		id: string;
		collectionId: string;
		collectionName: string;
		username: string;
		name?: string;
		avatar?: string;
		is_deleted?: boolean;
		[key: string]: unknown;
	}
	interface GamertagRow {
		id: string;
		user: string;
		tag: string;
		status: string;
		expand?: { user?: UserRow };
	}
	interface RosterRow {
		id: string;
		team: string;
		gamertag: string;
		is_owner: boolean;
		is_manager: boolean;
		joined_at: string;
		left_at: string | null;
		expand?: { gamertag?: GamertagRow };
	}
	interface TeamLogRow {
		id: string;
		team: string;
		event: string;
		actor: string;
		subject_user: string;
		subject_gamertag: string;
		payload_json: string;
		created: string;
		expand?: {
			actor?: UserRow;
			subject_user?: UserRow;
			subject_gamertag?: GamertagRow;
		};
	}
	interface MembershipRequestRow {
		id: string;
		team: string;
		user: string;
		direction: 'invited' | 'requested';
		status: 'pending' | 'accepted' | 'declined' | 'expired' | 'cancelled';
		initiated_by: string;
		created: string;
	}

	let team = $state<TeamRow | null>(null);
	let rosters = $state<RosterRow[]>([]);
	let timeline = $state<TeamLogRow[]>([]);
	let myPending = $state<MembershipRequestRow | null>(null);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let joinInFlight = $state(false);

	async function load() {
		loading = true;
		loadError = null;
		try {
			team = await pb.collection('teams').getFirstListItem<TeamRow>(`slug = "${slug}"`, {
				expand: 'created_by'
			});
			rosters = await pb.collection('rosters').getFullList<RosterRow>({
				filter: `team = "${team.id}"`,
				expand: 'gamertag,gamertag.user',
				sort: '-joined_at'
			});

			const eventFilter = ['member_added', 'member_left', 'member_removed', 'team_renamed']
				.map((e) => `event = "${e}"`)
				.join(' || ');
			const tl = await pb.collection('team_log').getList<TeamLogRow>(1, 20, {
				filter: `team = "${team.id}" && (${eventFilter})`,
				expand: 'actor,subject_user,subject_gamertag',
				sort: '-created'
			});
			timeline = tl.items;

			if (auth.isLoggedIn && auth.user) {
				try {
					myPending = await pb
						.collection('team_membership_requests')
						.getFirstListItem<MembershipRequestRow>(
							`team = "${team.id}" && user = "${auth.user.id}" && status = "pending"`
						);
				} catch {
					myPending = null;
				}
			}
		} catch (err) {
			loadError = describeAsyncError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
	});

	const founder = $derived(team?.expand?.created_by ?? null);
	const activeRoster = $derived(rosters.filter((r) => !r.left_at));
	const formerRoster = $derived(rosters.filter((r) => r.left_at));

	const iAmInRoster = $derived.by(() => {
		if (!auth.isLoggedIn || !auth.user) return false;
		return activeRoster.some((r) => r.expand?.gamertag?.user === auth.user!.id);
	});

	const canRequestToJoin = $derived.by(() => {
		if (!auth.isLoggedIn || !auth.user) return false;
		if (!team) return false;
		if (team.status === 'blocked') return false;
		if (iAmInRoster) return false;
		if (myPending) return false;
		return true;
	});

	async function requestToJoin() {
		if (!team) return;
		joinInFlight = true;
		try {
			await toastPromise(requestToJoinTeam(team.id), {
				loading: { title: 'Sending request', description: team.name },
				success: { title: 'Request sent', description: 'The team owners will see it.' },
				errorTitle: 'Could not send request'
			});
			await Promise.all([load(), notifications.refresh()]);
		} catch {
			// toast already shown
		} finally {
			joinInFlight = false;
		}
	}

	const iAmOwnerOrManager = $derived.by(() => {
		if (!auth.isLoggedIn || !auth.user) return false;
		return activeRoster.some(
			(r) => r.expand?.gamertag?.user === auth.user!.id && (r.is_owner || r.is_manager)
		);
	});

	let rosterActionBusy = $state<Record<string, boolean>>({});

	async function leaveMyRow(rosterID: string, teamName: string) {
		const ok = await confirmToast({
			title: `Leave ${teamName}?`,
			description: 'You will need a new invite or request to return.',
			confirmLabel: 'Leave team',
			type: 'warning'
		});
		if (!ok) return;
		rosterActionBusy = { ...rosterActionBusy, [rosterID]: true };
		try {
			await toastPromise(leaveRoster(rosterID), {
				loading: { title: 'Leaving', description: teamName },
				success: { title: 'You left the team', description: teamName },
				errorTitle: 'Leave failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const next = { ...rosterActionBusy };
			delete next[rosterID];
			rosterActionBusy = next;
		}
	}

	async function removeMember(rosterID: string, handle: string) {
		const ok = await confirmToast({
			title: `Remove @${handle}?`,
			description: 'They will be notified and lose access immediately.',
			confirmLabel: 'Remove',
			type: 'warning'
		});
		if (!ok) return;
		rosterActionBusy = { ...rosterActionBusy, [rosterID]: true };
		try {
			await toastPromise(removeRosterMember(rosterID), {
				loading: { title: 'Removing', description: `@${handle}` },
				success: { title: 'Removed', description: `@${handle}` },
				errorTitle: 'Remove failed'
			});
			await load();
		} catch {
			// toast already shown
		} finally {
			const next = { ...rosterActionBusy };
			delete next[rosterID];
			rosterActionBusy = next;
		}
	}

	function formatDate(iso: string | null): string {
		if (!iso) return '—';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return d.toLocaleDateString();
	}

	function formatDateTime(iso: string): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return d.toLocaleString();
	}

	function renderTeamLog(row: TeamLogRow): { title: string; subtitle: string } {
		const actor = row.expand?.actor;
		const subj = row.expand?.subject_user;
		const subjGt = row.expand?.subject_gamertag;
		const actorName = actor?.name?.trim() || actor?.username || '';
		const subjName = subj?.name?.trim() || subj?.username || '';
		switch (row.event) {
			case 'member_added': {
				let payload: { via?: string; gamertag?: string } = {};
				try {
					payload = JSON.parse(row.payload_json || '{}');
				} catch {
					/* ignore */
				}
				const handle = payload.gamertag ?? subjGt?.tag ?? subjName;
				const via = payload.via === 'invite' ? 'via invite' : 'via request';
				return {
					title: `@${handle} joined`,
					subtitle: `${subjName ? subjName + ' ' : ''}${via}${actorName ? ` — by ${actorName}` : ''}`
				};
			}
			case 'member_left':
				return {
					title: `@${subjGt?.tag ?? '?'} left`,
					subtitle: subjName ? `${subjName} self-left.` : 'Self-left.'
				};
			case 'member_removed':
				return {
					title: `@${subjGt?.tag ?? '?'} removed`,
					subtitle: `Removed${actorName ? ' by ' + actorName : ''}.`
				};
			case 'team_renamed': {
				let payload: { prev_name?: string; new_name?: string; by_admin?: boolean } = {};
				try {
					payload = JSON.parse(row.payload_json || '{}');
				} catch {
					/* ignore */
				}
				const tail = payload.by_admin ? ' (admin)' : '';
				return {
					title: `Renamed from "${payload.prev_name ?? '?'}"`,
					subtitle: `→ "${payload.new_name ?? team?.name ?? '?'}"${tail}`
				};
			}
			default:
				return { title: row.event, subtitle: '' };
		}
	}
</script>

<svelte:head>
	<title>{team?.name ?? slug} — Team</title>
</svelte:head>

<div class="mx-auto flex max-w-4xl flex-col gap-4">
	{#if loading}
		<Card>
			<div class="flex items-center gap-2 py-6 text-sm opacity-70">
				<LoaderIcon class="size-4 animate-spin" />
				Loading…
			</div>
		</Card>
	{:else if loadError}
		<Card class="preset-tonal-error!">
			<p class="text-sm">Failed to load: {loadError}</p>
		</Card>
	{:else if !team}
		<Card>
			<div class="py-6 text-center text-sm opacity-70">Team not found.</div>
		</Card>
	{:else}
		{@const t = team}
		<PageHeader title={t.name} description={`/${t.slug}/`}>
			{#snippet actions()}
				{#if t.status === 'blocked'}
					<span class="badge preset-tonal-error">blocked</span>
				{:else if t.status === 'pending'}
					<span class="badge preset-tonal-warning">pending review</span>
				{:else if t.status === 'allowed'}
					<span class="badge preset-tonal">awaiting approval</span>
				{/if}

				{#if canRequestToJoin}
					<button
						type="button"
						class="btn preset-filled-primary-500 btn-sm"
						onclick={requestToJoin}
						disabled={joinInFlight}
					>
						<LogInIcon class="size-4" />
						<span>Request to join</span>
					</button>
				{:else if myPending}
					<span class="badge preset-tonal-warning">
						<ClockIcon class="me-1 size-3.5" />
						{myPending.direction === 'invited' ? 'Invite pending' : 'Request pending'}
					</span>
				{/if}
			{/snippet}
		</PageHeader>

		{#if founder}
			<p class="text-sm opacity-75">
				Founded by
				{#if founder.username}
					<a href={resolve('/u/[username]', { username: founder.username })} class="anchor"
						>{founder.name?.trim() || founder.username}</a
					>
				{:else}
					<span>{founder.name?.trim() || '[unknown]'}</span>
				{/if}
				on {formatDate(t.created)}.
			</p>
		{/if}

		<section class="flex flex-col gap-2">
			<h2 class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70">
				<UsersIcon class="size-4" />
				Roster <span class="opacity-50">({activeRoster.length})</span>
			</h2>
			{#if activeRoster.length === 0}
				<Card>
					<p class="text-sm opacity-70">No active members.</p>
				</Card>
			{:else}
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each activeRoster as r (r.id)}
							{@const gt = r.expand?.gamertag}
							{@const u = gt?.expand?.user}
							<li class="flex items-center gap-3 px-4 py-3">
								{#if u}
									<UserAvatar user={u} size="size-9" />
								{:else}
									<div class="flex size-9 items-center justify-center rounded-full preset-tonal">
										<UserIcon class="size-4 opacity-60" />
									</div>
								{/if}
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm font-medium">
										{#if u?.username}
											<a href={resolve('/u/[username]', { username: u.username })} class="anchor"
												>@{gt?.tag ?? '?'}</a
											>
										{:else}
											@{gt?.tag ?? '?'}
										{/if}
									</p>
									<p class="truncate text-xs opacity-60">
										{u?.name?.trim() || u?.username || '[deleted user]'} · joined {formatDate(
											r.joined_at
										)}
									</p>
								</div>
								<div class="flex shrink-0 items-center gap-1">
									{#if r.is_owner}
										<span class="badge preset-tonal-warning" title="Owner">
											<CrownIcon class="size-3" />
										</span>
									{/if}
									{#if r.is_manager}
										<span class="badge preset-tonal-primary" title="Manager">
											<BriefcaseIcon class="size-3" />
										</span>
									{/if}
									{#if auth.user && u?.id === auth.user.id}
										<button
											type="button"
											class="btn preset-tonal btn-sm"
											onclick={() => leaveMyRow(r.id, t.name)}
											disabled={rosterActionBusy[r.id]}
											title="Leave team"
										>
											<LogOutIcon class="size-3.5" />
										</button>
									{:else if iAmOwnerOrManager}
										<button
											type="button"
											class="btn preset-tonal-error btn-sm"
											onclick={() => removeMember(r.id, gt?.tag ?? '?')}
											disabled={rosterActionBusy[r.id]}
											title="Remove from team"
										>
											<UserMinusIcon class="size-3.5" />
										</button>
									{/if}
								</div>
							</li>
						{/each}
					</ul>
				</Card>
			{/if}
		</section>

		{#if formerRoster.length > 0}
			<section class="flex flex-col gap-2">
				<h2
					class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70"
				>
					<HistoryIcon class="size-4" />
					Former members <span class="opacity-50">({formerRoster.length})</span>
				</h2>
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each formerRoster as r (r.id)}
							{@const gt = r.expand?.gamertag}
							{@const u = gt?.expand?.user}
							<li class="flex items-center gap-3 px-4 py-3 opacity-75">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm">
										{#if u?.username}
											<a href={resolve('/u/[username]', { username: u.username })} class="anchor"
												>@{gt?.tag ?? '?'}</a
											>
										{:else}
											@{gt?.tag ?? '?'}
										{/if}
									</p>
									<p class="truncate text-xs opacity-60">
										{formatDate(r.joined_at)} — {formatDate(r.left_at)}
									</p>
								</div>
							</li>
						{/each}
					</ul>
				</Card>
			</section>
		{/if}

		<section class="flex flex-col gap-2">
			<h2 class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70">
				<ClockIcon class="size-4" />
				Activity
			</h2>
			{#if timeline.length === 0}
				<Card>
					<p class="text-sm opacity-70">No recorded activity yet.</p>
				</Card>
			{:else}
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each timeline as t (t.id)}
							{@const r = renderTeamLog(t)}
							<li class="flex items-start justify-between gap-3 px-4 py-2.5">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm">{r.title}</p>
									{#if r.subtitle}
										<p class="truncate text-xs opacity-60">{r.subtitle}</p>
									{/if}
								</div>
								<p class="shrink-0 text-[11px] opacity-50">{formatDateTime(t.created)}</p>
							</li>
						{/each}
					</ul>
				</Card>
			{/if}
		</section>
	{/if}
</div>
