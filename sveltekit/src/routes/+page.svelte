<script lang="ts">
	import { resolve } from '$app/paths';
	import { APP_NAME } from '$lib/config/app';
	import { ServerIcon, PaletteIcon, MessageSquareIcon, ArrowRightIcon } from '@lucide/svelte';
	import type { Component } from 'svelte';

	interface Feature {
		icon: Component<{ class?: string }>;
		title: string;
		body: string;
	}

	const features: Feature[] = [
		{
			icon: ServerIcon,
			title: 'Go + PocketBase',
			body: 'Single-binary backend with auth, SQLite, REST, and hooks — ready to extend.'
		},
		{
			icon: PaletteIcon,
			title: 'Skeleton UI v4',
			body: 'Svelte 5 runes, Tailwind v4, and a themeable component library out of the box.'
		},
		{
			icon: MessageSquareIcon,
			title: 'Discord + WebSocket',
			body: 'Disgo bot and a JWT-authed WebSocket hub wired into the same process.'
		}
	];
</script>

<div class="mx-auto flex w-full max-w-5xl flex-col items-center gap-12 py-12">
	<!-- Hero -->
	<section class="space-y-6 text-center">
		<h1 class="h1">
			Welcome to <span class="text-primary-500">{APP_NAME}</span>
		</h1>
		<p class="mx-auto max-w-2xl text-lg opacity-70">
			A batteries-included starter that pairs a Go + PocketBase backend with a Skeleton UI SvelteKit
			frontend — plus a Discord bot and WebSocket hub, all in one binary.
		</p>
		<div class="flex flex-wrap items-center justify-center gap-3">
			<a href={resolve('/login/')} class="btn preset-filled">
				Get Started
				<ArrowRightIcon class="size-4" />
			</a>
		</div>
	</section>

	<!-- Features -->
	<section class="grid w-full gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each features as feature (feature.title)}
			{@const Icon = feature.icon}
			<div class="space-y-3 card p-6">
				<div
					class="flex size-10 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500"
				>
					<Icon class="size-5" />
				</div>
				<h3 class="h5">{feature.title}</h3>
				<p class="text-sm opacity-70">{feature.body}</p>
			</div>
		{/each}
	</section>
</div>
