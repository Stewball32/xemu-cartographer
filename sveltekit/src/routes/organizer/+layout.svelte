<script lang="ts">
	// Organizer shell — a clean, consistent sub-nav across the organizer tools
	// (creator, games). Replaces the bare per-page layout. Rendered inside the
	// app's root layout; the route group is already gated to organizers/admins by
	// +layout.ts.
	import { page } from '$app/state';
	import { SwordsIcon, LibraryIcon, WandSparklesIcon } from '@lucide/svelte';
	import type { Component } from 'svelte';

	let { children } = $props();

	interface Tab {
		label: string;
		href: string;
		icon: Component;
		match: (path: string) => boolean;
	}
	const tabs: Tab[] = [
		{
			label: 'Creator',
			href: '/organizer/creator/',
			icon: WandSparklesIcon,
			match: (p) => p.startsWith('/organizer/creator') || p.startsWith('/organizer/gametypes')
		},
		{
			label: 'Games',
			href: '/organizer/games/',
			icon: LibraryIcon,
			match: (p) => p.startsWith('/organizer/games')
		}
	];
	const path = $derived(page.url.pathname);
</script>

<div class="mx-auto flex w-full max-w-7xl flex-col gap-4 p-4 sm:gap-6 sm:p-6">
	<div class="flex items-center gap-2 border-b border-surface-200-800 pb-2">
		<span class="mr-2 flex items-center gap-2 text-sm font-semibold text-surface-500">
			<SwordsIcon class="size-4" /> Organizer
		</span>
		<!-- hrefs are static route ids from a local config; resolve() can't take a
		     variable, matching the pattern in NavPanel. -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<nav class="flex flex-wrap gap-1">
			{#each tabs as t (t.href)}
				{@const active = t.match(path)}
				<a
					href={t.href}
					class="btn btn-sm {active ? 'preset-filled-primary-500' : 'preset-tonal'}"
					aria-current={active ? 'page' : undefined}
				>
					<t.icon class="size-4" /><span>{t.label}</span>
				</a>
			{/each}
		</nav>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	</div>

	{@render children()}
</div>
