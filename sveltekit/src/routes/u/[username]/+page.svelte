<script lang="ts">
	// Public profile — M23c. Replaces the M6 placeholder.
	//
	// Shows: name + avatar + bio + location, approved/allowed gamertags
	// (blocked tags hidden from the public view), current + past team
	// memberships. Adds an "Invite to a team" picker visible to viewers
	// who own/manage one or more teams; submission hits
	// POST /api/teams/{teamId}/invite.
	//
	// Soft-deleted users still tombstone — kept the pre-M23 lookup pattern
	// because /u/ is the public face of an account that may have been
	// purged. The placeholder logic (isDeleted / lookupFailed flags) sits
	// alongside the new real render.

	import { onMount } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import {
		ArrowLeftIcon,
		BriefcaseIcon,
		CrownIcon,
		HistoryIcon,
		LoaderIcon,
		MailPlusIcon,
		MapPinIcon,
		TagIcon,
		UsersIcon
	} from '@lucide/svelte';
	import pb from '$lib/pocketbase';
	import { auth } from '$lib/stores/auth.svelte';
	import { describeAsyncError, toaster, toastPromise } from '$lib/stores/toaster';
	import { inviteToTeam } from '$lib/utils/membership-actions';
	import { notifications } from '$lib/stores/notifications.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';

	const username = $derived(page.params.username ?? '');

	interface UserRow {
		id: string;
		collectionId: string;
		collectionName: string;
		username: string;
		name?: string;
		bio?: string;
		location?: string;
		avatar?: string;
		is_deleted?: boolean;
		[key: string]: unknown;
	}
	interface GamertagRow {
		id: string;
		user: string;
		tag: string;
		status: string;
	}
	interface TeamRow {
		id: string;
		name: string;
		slug: string;
		status: string;
	}
	interface RosterRow {
		id: string;
		team: string;
		gamertag: string;
		is_owner: boolean;
		is_manager: boolean;
		joined_at: string;
		left_at: string | null;
		expand?: { team?: TeamRow; gamertag?: GamertagRow };
	}

	let user = $state<UserRow | null>(null);
	let lookupFailed = $state(false);
	let isDeleted = $state(false);
	let gamertags = $state<GamertagRow[]>([]);
	let memberships = $state<RosterRow[]>([]);
	let myOwnerTeams = $state<RosterRow[]>([]);
	let loading = $state(true);
	let inviteOpen = $state(false);
	let inviteTeam = $state('');
	let inviteInFlight = $state(false);

	async function load() {
		loading = true;
		try {
			user = await pb.collection('users').getFirstListItem<UserRow>(`username = "${username}"`);
		} catch {
			lookupFailed = true;
			loading = false;
			return;
		}
		if (!user) {
			lookupFailed = true;
			loading = false;
			return;
		}
		if (user.is_deleted) {
			isDeleted = true;
			loading = false;
			return;
		}

		try {
			gamertags = await pb.collection('gamertags').getFullList<GamertagRow>({
				filter: `user = "${user.id}" && status != "blocked"`,
				sort: 'tag'
			});
		} catch (err) {
			toaster.error({ title: 'Gamertags load failed', description: describeAsyncError(err) });
		}

		try {
			memberships = await pb.collection('rosters').getFullList<RosterRow>({
				filter: `gamertag.user = "${user.id}"`,
				expand: 'team,gamertag',
				sort: '-joined_at'
			});
		} catch (err) {
			toaster.error({ title: 'Memberships load failed', description: describeAsyncError(err) });
		}

		// Viewer-side check: which teams does the CURRENT user own/manage?
		// Drives the "Invite to a team" picker. Skip when viewing your own
		// profile — self-invites are blocked at the route level anyway.
		if (auth.isLoggedIn && auth.user && auth.user.id !== user.id) {
			try {
				myOwnerTeams = await pb.collection('rosters').getFullList<RosterRow>({
					filter: `gamertag.user = "${auth.user.id}" && left_at = null && (is_owner = true || is_manager = true)`,
					expand: 'team',
					sort: 'team.name'
				});
			} catch (err) {
				console.warn('owner-teams load failed:', describeAsyncError(err));
			}
		}

		loading = false;
	}

	onMount(() => {
		void load();
	});

	const activeMemberships = $derived(memberships.filter((r) => !r.left_at));
	const pastMemberships = $derived(memberships.filter((r) => r.left_at));

	// Deduplicate the viewer's owner/manager teams (multiple roster rows
	// under different gamertags can collide on team.id).
	const inviteOptions = $derived.by(() => {
		const seen = new SvelteSet<string>();
		const out: TeamRow[] = [];
		for (const r of myOwnerTeams) {
			const team = r.expand?.team;
			if (!team || seen.has(team.id) || team.status === 'blocked') continue;
			seen.add(team.id);
			out.push(team);
		}
		return out;
	});

	async function submitInvite() {
		if (!user || !inviteTeam) return;
		inviteInFlight = true;
		try {
			await toastPromise(inviteToTeam(inviteTeam, user.id), {
				loading: { title: 'Sending invite', description: user.username },
				success: { title: 'Invite sent', description: `@${user.username} will see it.` },
				errorTitle: 'Could not send invite'
			});
			inviteOpen = false;
			inviteTeam = '';
			await notifications.refresh();
		} catch {
			// toast already shown
		} finally {
			inviteInFlight = false;
		}
	}

	function formatDate(iso: string | null): string {
		if (!iso) return '—';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return d.toLocaleDateString();
	}

	const headerTitle = $derived(user?.name?.trim() || `@${user?.username ?? username}`);
	const subtitleParts = $derived.by(() => {
		const parts: string[] = [];
		if (user?.name && user.name.trim() !== '') parts.push(`@${user.username}`);
		if (user?.location && user.location.trim() !== '') parts.push(user.location.trim());
		return parts.join(' · ');
	});
</script>

<svelte:head>
	<title>@{username} — Profile</title>
</svelte:head>

<div class="mx-auto flex max-w-3xl flex-col gap-4">
	<a href={resolve('/')} class="inline-flex items-center gap-1 anchor text-sm">
		<ArrowLeftIcon class="size-4" />
		Back home
	</a>

	{#if loading}
		<Card>
			<div class="flex items-center gap-2 py-6 text-sm opacity-70">
				<LoaderIcon class="size-4 animate-spin" />
				Loading…
			</div>
		</Card>
	{:else if isDeleted}
		<h1 class="h2">[deleted user]</h1>
		<Card>
			<p class="text-sm opacity-80">
				This account was deleted. Past gamertag, roster, and team membership records remain attached
				to preserve game history, but the person's profile is gone.
			</p>
		</Card>
	{:else if lookupFailed || !user}
		<h1 class="h2">@{username}</h1>
		<Card>
			<p class="text-sm opacity-80">No account found for that username.</p>
		</Card>
	{:else}
		<PageHeader title={headerTitle} description={subtitleParts}>
			{#snippet actions()}
				{#if inviteOptions.length > 0}
					<button
						type="button"
						class="btn preset-tonal btn-sm"
						onclick={() => (inviteOpen = !inviteOpen)}
						title="Invite to one of your teams"
					>
						<MailPlusIcon class="size-4" />
						<span>Invite to team</span>
					</button>
				{/if}
			{/snippet}
		</PageHeader>

		<div class="flex items-start gap-4">
			<UserAvatar {user} size="size-20" />
			<div class="min-w-0 flex-1">
				{#if user.bio && user.bio.trim() !== ''}
					<p class="text-sm whitespace-pre-line">{user.bio}</p>
				{:else}
					<p class="text-sm italic opacity-60">No bio.</p>
				{/if}
				{#if user.location}
					<p class="mt-2 inline-flex items-center gap-1 text-xs opacity-60">
						<MapPinIcon class="size-3.5" />
						{user.location}
					</p>
				{/if}
			</div>
		</div>

		{#if inviteOpen}
			<Card>
				<div class="flex flex-col gap-2">
					<label class="text-xs font-semibold tracking-wide uppercase opacity-70" for="invite-team">
						Invite @{user.username} to
					</label>
					<select id="invite-team" class="select" bind:value={inviteTeam} disabled={inviteInFlight}>
						<option value="" disabled>Pick a team…</option>
						{#each inviteOptions as team (team.id)}
							<option value={team.id}>{team.name}</option>
						{/each}
					</select>
					<div class="flex justify-end gap-2">
						<button
							type="button"
							class="btn preset-tonal btn-sm"
							onclick={() => (inviteOpen = false)}
							disabled={inviteInFlight}
						>
							Cancel
						</button>
						<button
							type="button"
							class="btn preset-filled-primary-500 btn-sm"
							onclick={submitInvite}
							disabled={inviteInFlight || !inviteTeam}
						>
							<MailPlusIcon class="size-4" />
							Send invite
						</button>
					</div>
				</div>
			</Card>
		{/if}

		<section class="flex flex-col gap-2">
			<h2 class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70">
				<TagIcon class="size-4" />
				Gamertags <span class="opacity-50">({gamertags.length})</span>
			</h2>
			{#if gamertags.length === 0}
				<Card>
					<p class="text-sm opacity-70">No public gamertags.</p>
				</Card>
			{:else}
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each gamertags as g (g.id)}
							<li class="flex items-center justify-between gap-3 px-4 py-2.5">
								<span class="text-sm">@{g.tag}</span>
								{#if g.status === 'allowed'}
									<span class="badge preset-tonal text-[10px]">awaiting approval</span>
								{:else if g.status === 'pending'}
									<span class="badge preset-tonal-warning text-[10px]">pending review</span>
								{/if}
							</li>
						{/each}
					</ul>
				</Card>
			{/if}
		</section>

		<section class="flex flex-col gap-2">
			<h2 class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70">
				<UsersIcon class="size-4" />
				Teams <span class="opacity-50">({activeMemberships.length})</span>
			</h2>
			{#if activeMemberships.length === 0}
				<Card>
					<p class="text-sm opacity-70">Not currently on any team.</p>
				</Card>
			{:else}
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each activeMemberships as r (r.id)}
							{@const t = r.expand?.team}
							<li class="flex items-center justify-between gap-3 px-4 py-3">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm font-medium">
										{#if t}
											<a href={resolve('/teams/[slug]', { slug: t.slug })} class="anchor"
												>{t.name}</a
											>
										{:else}
											[unknown team]
										{/if}
									</p>
									<p class="text-xs opacity-60">
										as @{r.expand?.gamertag?.tag ?? '?'} · joined {formatDate(r.joined_at)}
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
								</div>
							</li>
						{/each}
					</ul>
				</Card>
			{/if}
		</section>

		{#if pastMemberships.length > 0}
			<section class="flex flex-col gap-2">
				<h2
					class="flex items-center gap-2 text-sm font-semibold tracking-wide uppercase opacity-70"
				>
					<HistoryIcon class="size-4" />
					Former teams <span class="opacity-50">({pastMemberships.length})</span>
				</h2>
				<Card size="flush">
					<ul class="divide-y divide-surface-200-800">
						{#each pastMemberships as r (r.id)}
							{@const t = r.expand?.team}
							<li class="flex items-center justify-between gap-3 px-4 py-2.5 opacity-75">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm">
										{#if t}
											<a href={resolve('/teams/[slug]', { slug: t.slug })} class="anchor"
												>{t.name}</a
											>
										{:else}
											[unknown team]
										{/if}
									</p>
									<p class="text-xs opacity-60">
										as @{r.expand?.gamertag?.tag ?? '?'} · {formatDate(r.joined_at)} —
										{formatDate(r.left_at)}
									</p>
								</div>
							</li>
						{/each}
					</ul>
				</Card>
			</section>
		{/if}
	{/if}
</div>
