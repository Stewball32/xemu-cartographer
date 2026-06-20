<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { statusStrip } from '$lib/utils/overlay-view';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	const vm = $derived(statusStrip(feed.game, feed.scenario));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			// game = config + team scores; scenario = map name.
			classes: ['game', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const live = $derived(vm.phase === 'live');
</script>

<svelte:head>
	<style>
		html,
		body {
			background: transparent !important;
			background-image: none !important;
			overflow: hidden !important;
			margin: 0 !important;
		}
		body::before,
		body::after {
			display: none !important;
			content: none !important;
		}
	</style>
</svelte:head>

<div class="wrap" style="--scale: {data.scale}">
	{#if feed.missingToken}
		<div class="status">
			No overlay token. Mint one at <code>/overlays/manage/</code> or append <code>?mock=1</code>.
		</div>
	{:else if !feed.connected}
		<div class="status">Connecting…</div>
	{:else}
		<div class="strip">
			<div class="seg left">
				<span class="phase" class:live>
					{vm.phase === 'live' ? 'LIVE' : vm.phase === 'ready' ? 'SETUP' : 'IDLE'}
				</span>
				{#if feed.mock}<span class="badge">MOCK</span>{/if}
			</div>

			<div class="seg center">
				{#if vm.isTeamGame && vm.teams.length >= 2}
					<span class="tscore" style="--team: {vm.teams[0].color}">
						<span class="tname">{vm.teams[0].name}</span>
						<span class="num">{vm.hasScores ? vm.teams[0].score : '—'}</span>
					</span>
					<span class="vs">:</span>
					<span class="tscore" style="--team: {vm.teams[1].color}">
						<span class="num">{vm.hasScores ? vm.teams[1].score : '—'}</span>
						<span class="tname">{vm.teams[1].name}</span>
					</span>
				{:else}
					<span class="gt-center">{vm.variant || vm.gametype || 'Match'}</span>
				{/if}
			</div>

			<div class="seg right">
				{#if vm.isTeamGame && vm.teams.length >= 2}
					<span class="meta">{vm.variant || vm.gametype}</span>
				{/if}
				{#if vm.map}<span class="map">{vm.map}</span>{/if}
				{#if vm.scoreLimit > 0}<span class="limit">to {vm.scoreLimit}</span>{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.wrap {
		position: fixed;
		inset: 0;
		display: flex;
		justify-content: center;
		align-items: flex-start;
		padding: 1rem;
		font-family: 'Rajdhani', 'Inter', system-ui, sans-serif;
		color: #fff;
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: top center;
	}

	.status {
		background: rgba(8, 10, 16, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.12);
		padding: 0.55rem 1rem;
		border-radius: 0.5rem;
		font-size: 0.9rem;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.status code {
		font-family: 'JetBrains Mono', monospace;
		font-size: 0.82em;
		background: rgba(255, 255, 255, 0.1);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
	}

	.strip {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: 1rem;
		min-width: 34rem;
		background: rgba(8, 10, 16, 0.78);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 0.6rem;
		padding: 0.4rem 0.9rem;
		backdrop-filter: blur(2px);
	}

	.seg {
		display: flex;
		align-items: center;
		gap: 0.6rem;
	}
	.seg.left {
		justify-content: flex-start;
	}
	.seg.center {
		justify-content: center;
		gap: 0.8rem;
	}
	.seg.right {
		justify-content: flex-end;
		gap: 0.7rem;
		font-size: 0.8rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		opacity: 0.85;
	}

	.phase {
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.16em;
		padding: 0.12rem 0.5rem;
		border-radius: 0.25rem;
		background: rgba(255, 255, 255, 0.14);
	}
	.phase.live {
		background: #e3413f;
		box-shadow: 0 0 10px rgba(227, 65, 63, 0.6);
		animation: pulse 1.8s ease-in-out infinite;
	}
	@keyframes pulse {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.65;
		}
	}

	.badge {
		font-size: 0.62rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
	}

	.tscore {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.tname {
		font-weight: 700;
		font-size: 0.85rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--team);
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95);
	}
	.num {
		font-size: 1.6rem;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		line-height: 1;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95);
	}
	.vs {
		opacity: 0.5;
		font-size: 1.1rem;
		font-weight: 700;
	}
	.gt-center {
		font-size: 1.05rem;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95);
	}

	.map {
		font-weight: 700;
	}
	.limit,
	.meta {
		opacity: 0.75;
	}
</style>
