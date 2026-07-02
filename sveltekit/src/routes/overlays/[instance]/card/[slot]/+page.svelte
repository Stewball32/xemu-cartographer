<script lang="ts">
	// Single-player SPOTLIGHT card browser source — one BroadcastPlayerCard for the
	// roster slot in the URL (/overlays/<i>/card/<slot>/). A larger standalone
	// version of the cards-strip cell for a "featured player" corner source.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildScoreboard, teamMeta } from '$lib/utils/overlay-view';
	import type { PlayerRow } from '$lib/utils/overlay-view';
	import { broadcastTheme, themeVars } from '$lib/components/broadcast/theme';
	import BroadcastPlayerCard from '$lib/components/broadcast/BroadcastPlayerCard.svelte';
	import { createProfileLookup } from '$lib/stores/broadcast-profiles.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	const theme = $derived(broadcastTheme(data.game));
	const vm = $derived(buildScoreboard(feed.game, feed.tick));
	const profiles = createProfileLookup();

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game', 'tick']
		})
	);
	onDestroy(() => feed.stop());

	// Flatten teams/FFA back to one roster and pick the requested slot.
	const roster = $derived<PlayerRow[]>(
		vm.isTeamGame ? vm.teams.flatMap((t) => t.players) : vm.players
	);
	const player = $derived(roster.find((p) => p.index === data.slot) ?? null);

	$effect(() => {
		if (player) profiles.ensure([player.name], feed.mock);
	});
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

<div class="stage" style="--scale: {data.scale}; {themeVars(theme)}" data-game={data.game}>
	{#if feed.missingToken}
		<div class="status">
			No overlay token. Mint one at <code>/overlays/manage/</code> or append
			<code>?mock=1</code> to preview.
		</div>
	{:else if !feed.connected}
		<div class="status">Connecting…</div>
	{:else if !player}
		<div class="status">Waiting for player in slot {data.slot}…</div>
	{:else}
		<div class="spot">
			{#if feed.mock}<span class="badge">MOCK</span>{/if}
			<BroadcastPlayerCard
				{player}
				game={data.game}
				teamColor={teamMeta(player.team).color}
				isTeamGame={vm.isTeamGame}
				teamLabel={vm.isTeamGame ? teamMeta(player.team).name : undefined}
				profile={profiles.get(player.name)}
				size={210}
				hasTick={vm.hasTick}
				hasScores={vm.hasScores}
			/>
		</div>
	{/if}
</div>

<style>
	/* Flush / content-sized — see the scoreboard route for the rationale. */
	.stage {
		display: flex;
		width: fit-content;
		font-family: var(--bc-font);
		color: var(--bc-ink);
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: top left;
	}
	.spot {
		position: relative;
		display: inline-flex;
		flex-direction: column;
		align-items: center;
		gap: 0.35rem;
	}
	.badge {
		font-size: 0.6rem;
		font-weight: 800;
		letter-spacing: 0.14em;
		background: var(--bc-accent);
		color: #12100a;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
	}
	.status {
		display: inline-block;
		background: rgba(8, 10, 16, 0.72);
		border: 1px solid rgba(255, 255, 255, 0.12);
		padding: 0.6rem 1rem;
		border-radius: 0.5rem;
		font-size: 0.95rem;
		color: #fff;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.status code {
		font-family: 'JetBrains Mono', monospace;
		font-size: 0.82em;
		background: rgba(255, 255, 255, 0.1);
		padding: 0.05em 0.35em;
		border-radius: 0.25rem;
	}
</style>
