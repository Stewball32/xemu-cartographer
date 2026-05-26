<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import {
		ActivityIcon,
		BoxIcon,
		DatabaseIcon,
		KeyIcon,
		UserIcon,
		UsersIcon
	} from '@lucide/svelte';
	import type { Component } from 'svelte';
	import pb from '$lib/pocketbase';
	import { adminGet } from '$lib/utils/admin-api';
	import type { ContainerInfo } from '$lib/types/containers';
	import type { ScraperInfo } from '$lib/types/scraper';

	let podCount = $state<number | null>(null);
	let scraperCount = $state<number | null>(null);
	let userCount = $state<number | null>(null);

	async function loadStats() {
		const [pods, scrapers, users] = await Promise.allSettled([
			adminGet<ContainerInfo[] | null>('containers'),
			adminGet<ScraperInfo[] | null>('scraper'),
			pb.collection('users').getList(1, 1)
		]);
		if (pods.status === 'fulfilled') podCount = (pods.value ?? []).length;
		if (scrapers.status === 'fulfilled') scraperCount = (scrapers.value ?? []).length;
		if (users.status === 'fulfilled') userCount = users.value.totalItems;
	}

	onMount(loadStats);

	type Card = {
		title: string;
		description: string;
		icon: Component;
		href: string;
		status: 'live' | 'placeholder';
		milestone?: string;
	};

	const cards: Card[] = [
		{
			title: 'Pod',
			description: 'Manage xemu + browser container pairs.',
			icon: BoxIcon,
			href: resolve('/admin/pod/'),
			status: 'live'
		},
		{
			title: 'Players',
			description: 'Moderate user gamertags.',
			icon: UserIcon,
			href: resolve('/admin/players/'),
			status: 'live',
			milestone: 'M07'
		},
		{
			title: 'Rosters',
			description: 'Team metadata + roster history.',
			icon: UsersIcon,
			href: resolve('/admin/rosters/'),
			status: 'live',
			milestone: 'M07'
		},
		{
			title: 'Roles',
			description: 'Role + permission management.',
			icon: KeyIcon,
			href: resolve('/admin/roles/'),
			status: 'placeholder',
			milestone: 'M08'
		},
		{
			title: 'Games',
			description: 'Captured games + series persistence.',
			icon: DatabaseIcon,
			href: resolve('/admin/games/'),
			status: 'placeholder',
			milestone: 'M13'
		}
	];

	function fmt(value: number | null): string {
		return value === null ? '—' : String(value);
	}
</script>

<div class="mx-auto flex max-w-6xl flex-col gap-4">
	<header class="flex flex-wrap items-end justify-between gap-2">
		<h1 class="h2">Admin</h1>
	</header>

	<div class="grid grid-cols-3 gap-2">
		<div class="flex items-center gap-3 card preset-tonal p-3">
			<BoxIcon class="size-5 text-surface-600-400" />
			<div>
				<div class="text-xs text-surface-600-400 uppercase">Pods</div>
				<div class="font-mono text-lg">{fmt(podCount)}</div>
			</div>
		</div>
		<div class="flex items-center gap-3 card preset-tonal p-3">
			<ActivityIcon class="size-5 text-surface-600-400" />
			<div>
				<div class="text-xs text-surface-600-400 uppercase">Scrapers</div>
				<div class="font-mono text-lg">{fmt(scraperCount)}</div>
			</div>
		</div>
		<div class="flex items-center gap-3 card preset-tonal p-3">
			<UsersIcon class="size-5 text-surface-600-400" />
			<div>
				<div class="text-xs text-surface-600-400 uppercase">Users</div>
				<div class="font-mono text-lg">{fmt(userCount)}</div>
			</div>
		</div>
	</div>

	<!-- eslint-disable svelte/no-navigation-without-resolve -->
	<div class="grid gap-3 sm:grid-cols-2">
		{#each cards as card (card.href)}
			<a
				href={card.href}
				class="flex flex-col gap-2 card preset-tonal p-4 hover:preset-tonal-primary"
			>
				<div class="flex items-center gap-2">
					<card.icon class="size-5" />
					<span class="font-medium">{card.title}</span>
					{#if card.status === 'live'}
						<span class="ms-auto badge preset-tonal-success text-xs">Live</span>
					{:else}
						<span class="ms-auto badge preset-tonal-warning text-xs">{card.milestone}</span>
					{/if}
				</div>
				<p class="text-sm text-surface-600-400">{card.description}</p>
			</a>
		{/each}
	</div>
	<!-- eslint-enable svelte/no-navigation-without-resolve -->
</div>
