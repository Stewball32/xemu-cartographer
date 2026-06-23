<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildKillFeed } from '$lib/utils/overlay-view';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	// game = match-presence (so we can show "waiting"); event = the death stream.
	const vm = $derived(buildKillFeed(feed.events, data.max));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game', 'event']
		})
	);
	onDestroy(() => feed.stop());

	// Cause → verb shown when the kill is self-inflicted (no killer to credit).
	function selfVerb(cause: string): string {
		if (cause === 'fall') return 'fell';
		if (cause === 'suicide') return 'self-destructed';
		return 'died';
	}
</script>

<svelte:head>
	<!-- Transparent OBS canvas, no scrollbars; neutralise the theme body
	     decorations so nothing composites into the capture. -->
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
		<div class="feed">
			{#if feed.mock}<span class="badge">MOCK</span>{/if}
			{#if vm.count === 0}
				<div class="empty">Waiting for kills…</div>
			{:else}
				{#each vm.rows as r (r.key)}
					<div class="kill" class:tk={r.teamKill}>
						{#if r.selfInflicted}
							<span class="name" style="--c: {r.victimColor}">{r.victimName}</span>
							<span class="verb">{selfVerb(r.cause)}</span>
						{:else}
							<span class="name" style="--c: {r.killerColor}">{r.killerName}</span>
							<span class="weapon">
								{#if r.teamKill}<span class="tkflag" title="Betrayal">✖</span>{/if}
								{r.weapon || '✕'}
							</span>
							<span class="name" style="--c: {r.victimColor}">{r.victimName}</span>
						{/if}
					</div>
				{/each}
			{/if}
		</div>
	{/if}
</div>

<style>
	.wrap {
		position: fixed;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		padding: 1.25rem;
		font-family: 'Rajdhani', 'Inter', system-ui, sans-serif;
		color: #fff;
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: top right;
	}

	.status {
		align-self: flex-start;
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

	.feed {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 0.3rem;
		width: 21rem;
	}
	.badge {
		font-size: 0.6rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		background: rgba(224, 163, 46, 0.9);
		color: #1a1300;
		padding: 0.1rem 0.4rem;
		border-radius: 0.25rem;
		margin-bottom: 0.1rem;
	}
	.empty {
		font-size: 0.8rem;
		opacity: 0.6;
		background: rgba(8, 10, 16, 0.55);
		padding: 0.3rem 0.6rem;
		border-radius: 0.35rem;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}

	.kill {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0.28rem 0.6rem;
		background: rgba(8, 10, 16, 0.66);
		border-right: 3px solid rgba(255, 255, 255, 0.15);
		border-radius: 0.35rem;
		font-size: 0.92rem;
		white-space: nowrap;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
		animation: slidein 0.25s ease-out;
	}
	.kill.tk {
		border-right-color: #e3413f;
	}
	@keyframes slidein {
		from {
			opacity: 0;
			transform: translateX(10px);
		}
		to {
			opacity: 1;
			transform: translateX(0);
		}
	}

	.name {
		font-weight: 700;
		color: var(--c);
	}
	.verb {
		opacity: 0.75;
		font-style: italic;
	}
	.weapon {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		font-size: 0.78rem;
		text-transform: capitalize;
		opacity: 0.9;
		padding: 0 0.15rem;
	}
	.tkflag {
		color: #ff8a8a;
		font-size: 0.7rem;
	}
</style>
