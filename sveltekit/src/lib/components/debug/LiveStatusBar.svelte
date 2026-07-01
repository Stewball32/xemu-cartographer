<script lang="ts" module>
	// One entry in the FEED strip — the debug analog of the live visualizer's
	// FEED capability grid (which envelope classes have delivered data).
	export type FeedClass = { label: string; live: boolean; title?: string };
</script>

<script lang="ts">
	import {
		HashIcon,
		MapIcon,
		RadioTowerIcon,
		ServerIcon,
		TagIcon,
		UsersIcon
	} from '@lucide/svelte';
	import type { Phase } from '$lib/types/scraper';

	// Cohesive "live scraper" identity/status bar for the pod debug page.
	// Mirrors the layout language of the live visualizer view
	// (/visualizer/[instance]/): a top identity line — map · gametype · phase —
	// with connection / players / engine-tick on the right, and a FEED strip of
	// per-class activity chips. Built on Skeleton v4 tokens so it stays
	// consistent with the rest of the admin shell.
	let {
		map,
		gametype,
		title,
		xboxName,
		phase,
		running,
		connected,
		lastError = null,
		players,
		scoreSummary,
		engineTick,
		feed
	}: {
		map: string;
		gametype: string;
		title: string;
		xboxName: string;
		phase: Phase;
		running: boolean;
		connected: boolean;
		lastError?: string | null;
		players?: number;
		scoreSummary?: string;
		engineTick?: number;
		feed: FeedClass[];
	} = $props();

	// Phase badge colour: dim when the scraper isn't running (the phase value is
	// stale), else live=success / ready=warning / idle=tonal — same mapping the
	// page's other phase surfaces use.
	const phaseBadge = $derived(
		!running
			? 'preset-tonal'
			: phase === 'live'
				? 'preset-filled-success-500'
				: phase === 'ready'
					? 'preset-filled-warning-500'
					: 'preset-tonal'
	);

	// "live" = a running scraper in the live phase with an open socket, i.e. the
	// visualizer's ● LIVE state. Drives the pulsing connection dot.
	const live = $derived(running && connected && phase === 'live');
	const connLabel = $derived(connected ? (live ? 'live' : 'connected') : 'offline');
</script>

<section class="flex flex-col gap-3 card preset-tonal p-3 sm:p-4" aria-label="Live scraper status">
	<!-- Identity: map · gametype · phase | (right) players · tick · connection -->
	<div class="flex flex-wrap items-center gap-x-3 gap-y-2">
		<div class="flex min-w-0 flex-wrap items-baseline gap-x-2.5 gap-y-1">
			<MapIcon class="text-surface-600-300 size-4 shrink-0 self-center" />
			<span class="truncate text-lg leading-none font-bold">{map || '—'}</span>
			{#if gametype}
				<span class="text-surface-600-300 text-xs font-medium tracking-wide uppercase">
					{gametype}
				</span>
			{/if}
			<span class="badge {phaseBadge} text-[10px] uppercase">{phase}</span>
			{#if scoreSummary}
				<span class="font-mono text-sm text-surface-800-200 tabular-nums">{scoreSummary}</span>
			{/if}
		</div>

		<div class="flex flex-wrap items-center gap-2 text-xs sm:ms-auto">
			{#if players !== undefined}
				<span
					class="text-surface-700-200 inline-flex items-center gap-1"
					title="players in current roster"
				>
					<UsersIcon class="size-3.5" />
					<span class="font-mono tabular-nums">{players}</span>
				</span>
			{/if}
			{#if engineTick}
				<span
					class="text-surface-600-300 inline-flex items-center gap-1 font-mono tabular-nums"
					title="latest engine tick"
				>
					<HashIcon class="size-3.5" />{engineTick}
				</span>
			{/if}
			<span
				class="badge {connected
					? 'preset-filled-success-500'
					: 'preset-tonal-error'} gap-1.5 text-[11px] uppercase"
				title={lastError ?? (connected ? 'websocket connected' : 'websocket disconnected')}
			>
				<span class="size-1.5 rounded-full bg-current {live ? 'animate-pulse' : ''}"></span>
				{connLabel}
			</span>
		</div>
	</div>

	<!-- Meta: xbe title + Xbox console name (kept from the old stat tiles). -->
	<div class="text-surface-600-300 flex flex-wrap gap-x-4 gap-y-1 text-xs">
		<span class="inline-flex items-center gap-1">
			<TagIcon class="size-3.5" /> title
			<span class="font-mono text-surface-800-200">{title || '—'}</span>
		</span>
		<span class="inline-flex items-center gap-1">
			<ServerIcon class="size-3.5" /> xbox
			<span class="font-mono text-surface-800-200">{xboxName || '—'}</span>
		</span>
	</div>

	<!-- FEED: which per-instance envelope classes have delivered data. Direct
	     analog of the visualizer's FEED capability grid. -->
	<div class="flex flex-wrap items-center gap-1.5">
		<span
			class="text-surface-600-300 mr-1 inline-flex items-center gap-1 text-[10px] font-semibold tracking-wide uppercase"
		>
			<RadioTowerIcon class="size-3.5" /> feed
		</span>
		{#each feed as f (f.label)}
			<span
				class="badge {f.live
					? 'preset-filled-success-500'
					: 'preset-tonal'} font-mono text-[11px] lowercase"
				class:opacity-60={!f.live}
				title={f.title}
			>
				{f.label}
			</span>
		{/each}
	</div>
</section>
