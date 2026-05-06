<script lang="ts">
	import { resolve } from '$app/paths';
	import { APP_NAME } from '$lib/config/app';
	import { BoxIcon, ActivityIcon, MonitorIcon, ArrowRightIcon } from '@lucide/svelte';
	import type { Component } from 'svelte';

	interface Pillar {
		icon: Component<{ class?: string }>;
		title: string;
		body: string;
	}

	const pillars: Pillar[] = [
		{
			icon: BoxIcon,
			title: 'Neutral host containers',
			body: 'Spin up xemu instances as managed neutral hosts for matches — provisioned, started, and torn down on demand.'
		},
		{
			icon: ActivityIcon,
			title: 'Live game state',
			body: "Read the running game's memory in real time and stream what's happening on the field as the match plays out."
		},
		{
			icon: MonitorIcon,
			title: 'Overlays & records',
			body: 'Drive spectator and streaming overlays from live data, and keep records of games, teams, and players over time.'
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
			Tooling for hosting, observing, and recording original Xbox matches running in xemu — with a
			focus on Halo: CE and Halo 2.
		</p>
		<p class="text-sm opacity-60">Early alpha — pieces are landing one at a time.</p>
		<div class="flex flex-wrap items-center justify-center gap-3">
			<a href={resolve('/login/')} class="btn preset-filled">
				Get Started
				<ArrowRightIcon class="size-4" />
			</a>
		</div>
	</section>

	<!-- Features -->
	<section class="grid w-full gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each pillars as pillar (pillar.title)}
			{@const Icon = pillar.icon}
			<div class="space-y-3 card p-6">
				<div
					class="flex size-10 items-center justify-center rounded-lg bg-primary-500/10 text-primary-500"
				>
					<Icon class="size-5" />
				</div>
				<h3 class="h5">{pillar.title}</h3>
				<p class="text-sm opacity-70">{pillar.body}</p>
			</div>
		{/each}
	</section>
</div>
