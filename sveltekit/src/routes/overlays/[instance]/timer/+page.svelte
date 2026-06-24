<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchClock } from '$lib/utils/overlay-view';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	// game alone carries engine_tick + config (time/score limits) — that's all the
	// clock needs, so subscribe to the single class.
	const vm = $derived(matchClock(feed.game));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game']
		})
	);
	onDestroy(() => feed.stop());

	const phaseLabel = $derived(
		vm.phase === 'live' ? 'LIVE' : vm.phase === 'ready' ? 'SETUP' : 'IDLE'
	);
</script>

<svelte:head>
	<!-- Transparent OBS canvas, no scrollbars; neutralise the theme's body
	     background + ::before/::after decorations so nothing composites in. -->
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
			No overlay token. Mint one at <code>/studio/</code> or append <code>?mock=1</code>.
		</div>
	{:else if !feed.connected}
		<div class="status">Connecting…</div>
	{:else}
		<div class="clock" class:live={vm.live}>
			<div class="top">
				<span class="phase" class:live={vm.live}>{phaseLabel}</span>
				{#if feed.mock}<span class="badge">MOCK</span>{/if}
				{#if vm.direction === 'down'}<span class="dir" title="Counting down to the time limit"
						>▾</span
					>{/if}
			</div>
			<div class="time">{vm.label}</div>
			<div class="sub">
				{#if vm.gametype}<span class="gt">{vm.gametype}</span>{/if}
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

	.clock {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.15rem;
		min-width: 9rem;
		padding: 0.5rem 1.4rem 0.65rem;
		background: rgba(8, 10, 16, 0.78);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-top: 3px solid rgba(255, 255, 255, 0.25);
		border-radius: 0.6rem;
		backdrop-filter: blur(2px);
	}
	.clock.live {
		border-top-color: #e3413f;
	}

	.top {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}
	.phase {
		font-size: 0.66rem;
		font-weight: 700;
		letter-spacing: 0.16em;
		padding: 0.1rem 0.45rem;
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
		font-size: 0.6rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
	}
	.dir {
		font-size: 0.7rem;
		opacity: 0.7;
	}

	.time {
		font-size: 2.6rem;
		font-weight: 700;
		line-height: 1;
		font-variant-numeric: tabular-nums;
		letter-spacing: 0.02em;
		text-shadow: 0 2px 6px rgba(0, 0, 0, 0.95);
	}

	.sub {
		display: flex;
		align-items: baseline;
		gap: 0.5rem;
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		opacity: 0.82;
	}
	.gt {
		font-weight: 700;
	}
	.limit {
		opacity: 0.75;
	}
</style>
