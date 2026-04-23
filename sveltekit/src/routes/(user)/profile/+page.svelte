<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { Avatar } from '@skeletonlabs/skeleton-svelte';
	import {
		ShieldCheckIcon,
		MapPinIcon,
		CalendarIcon,
		MailIcon,
		SettingsIcon,
		UserIcon
	} from '@lucide/svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { getFileURL } from '$lib/utils/files';
	import { OAUTH_PROVIDERS } from '$lib/config/app';

	let linkedAuths = $state<Array<Record<string, string>>>([]);

	onMount(async () => {
		if (auth.user) {
			try {
				linkedAuths = await auth.listExternalAuths(auth.user.id);
			} catch {
				linkedAuths = [];
			}
		}
	});

	const avatarUrl = $derived(
		auth.user ? getFileURL(auth.user, 'avatar', { thumb: '160x160' }) : null
	);

	const initials = $derived(
		auth.user?.name
			?.split(' ')
			.map((n: string) => n[0])
			.join('')
			.toUpperCase()
			.slice(0, 2) ?? '?'
	);

	const memberSince = $derived(
		auth.user
			? new Date(auth.user.created).toLocaleDateString('en-US', {
					month: 'long',
					year: 'numeric'
				})
			: ''
	);
</script>

<h1 class="mb-6 h2">Profile</h1>

<div class="max-w-2xl space-y-6">
	<!-- Profile Header -->
	<div class="overflow-hidden card">
		<div class="h-20 bg-linear-to-r from-primary-500 to-secondary-500"></div>
		<div class="px-6 pb-6">
			<div class="-mt-10 flex items-end gap-4">
				<Avatar class="size-20 ring-4 ring-surface-100-900">
					{#if avatarUrl}
						<Avatar.Image src={avatarUrl} />
					{/if}
					<Avatar.Fallback>{initials}</Avatar.Fallback>
				</Avatar>
				<div class="flex-1 pb-1">
					<div class="flex items-center gap-2">
						<h2 class="h3">{auth.user?.name || auth.user?.username}</h2>
						{#if auth.user?.verified}
							<ShieldCheckIcon class="size-5 text-success-500" />
						{/if}
					</div>
					{#if auth.user?.name && auth.user?.username}
						<p class="text-sm opacity-70">@{auth.user.username}</p>
					{/if}
				</div>
				<a href={resolve('/settings/')} class="btn preset-tonal btn-sm">
					<SettingsIcon class="size-4" />
					<span>Edit Profile</span>
				</a>
			</div>

			{#if auth.user?.bio}
				<p class="mt-4 text-sm">{auth.user.bio}</p>
			{/if}

			{#if auth.user?.location}
				<div class="mt-2 flex items-center gap-1.5 text-sm opacity-70">
					<MapPinIcon class="size-4" />
					<span>{auth.user.location}</span>
				</div>
			{/if}
		</div>
	</div>

	<!-- Details -->
	<div class="space-y-4 card p-6">
		<h2 class="h4">Details</h2>

		<div class="space-y-3">
			<div class="flex items-center gap-3">
				<UserIcon class="size-4 opacity-50" />
				<span class="text-sm">@{auth.user?.username}</span>
			</div>

			<div class="flex items-center gap-3">
				<MailIcon class="size-4 opacity-50" />
				{#if auth.user?.emailVisibility}
					<span class="text-sm">{auth.user.email}</span>
				{:else}
					<span class="text-sm opacity-50">Email hidden</span>
				{/if}
			</div>

			<div class="flex items-center gap-3">
				<CalendarIcon class="size-4 opacity-50" />
				<span class="text-sm">Member since {memberSince}</span>
			</div>
		</div>
	</div>

	<!-- Connected Accounts -->
	<div class="space-y-4 card p-6">
		<div class="flex items-center justify-between">
			<h2 class="h4">Connected Accounts</h2>
			<a href={resolve('/settings/')} class="btn preset-tonal btn-sm">Manage</a>
		</div>

		{#if linkedAuths.length === 0}
			<p class="text-sm opacity-50">No connected accounts.</p>
		{:else}
			<div class="flex flex-wrap gap-3">
				{#each linkedAuths as { provider } (provider)}
					{@const meta = OAUTH_PROVIDERS[provider]}
					{#if meta}
						<div class="flex items-center gap-2 rounded-md border border-surface-300-700 px-3 py-2">
							<img src={meta.icon} alt={meta.label} class="size-5" />
							<span class="text-sm font-medium">{meta.label}</span>
						</div>
					{/if}
				{/each}
			</div>
		{/if}
	</div>
</div>
