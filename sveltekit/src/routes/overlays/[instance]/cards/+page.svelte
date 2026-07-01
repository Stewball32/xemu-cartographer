<script lang="ts">
	// Broadcast PLAYER-CARDS browser source. A bottom strip of per-player cards —
	// each a Spartan tinted by the player's armor colour (H2 also shows the
	// player's emblem), with gamertag, K/D/A and score. Team games cluster into
	// Red / Blue groups with a score header; FFA is one score-sorted row.
	//
	// Reuses the shared overlay feed + buildScoreboard (for the joined roster+tick
	// rows) + the themed BroadcastPlayerCard. H2 emblem art comes from the mock in
	// preview; live H2 emblems await the H2 scraper (see the M28 milestone note).
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { buildScoreboard, teamMeta } from '$lib/utils/overlay-view';
	import type { PlayerRow } from '$lib/utils/overlay-view';
	import { broadcastTheme, themeVars } from '$lib/components/broadcast/theme';
	import BroadcastPlayerCard from '$lib/components/broadcast/BroadcastPlayerCard.svelte';
	import { mockAppearance } from '$lib/utils/overlay-mock';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const feed = createOverlayFeed();
	const theme = $derived(broadcastTheme(data.game));
	const vm = $derived(buildScoreboard(feed.game, feed.tick));

	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game', 'tick']
		})
	);
	onDestroy(() => feed.stop());

	// H2 emblems: mock supplies them in preview; live H2 has no emblem feed yet.
	function appearanceFor(idx: number) {
		return data.game === 'h2' && feed.mock ? mockAppearance(idx) : undefined;
	}

	interface Cluster {
		key: number;
		name: string;
		color: string;
		score: number;
		players: PlayerRow[];
	}
	const clusters = $derived.by((): Cluster[] => {
		if (vm.isTeamGame) {
			return vm.teams.map((t) => ({
				key: t.team,
				name: t.name,
				color: t.color,
				score: t.score,
				players: t.players
			}));
		}
		return [{ key: -1, name: '', color: theme.accent, score: 0, players: vm.players }];
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
	{:else if vm.playerCount === 0}
		<div class="status">Waiting for match…</div>
	{:else}
		<div class="strip">
			{#if feed.mock}<span class="badge">MOCK</span>{/if}
			<div class="clusters">
				{#each clusters as c (c.key)}
					<section class="cluster">
						{#if c.name}
							<header class="cluster-head" style="--team: {c.color}">
								<span class="c-name">{c.name}</span>
								{#if vm.hasScores}<span class="c-score">{c.score}</span>{/if}
							</header>
						{/if}
						<div class="cards">
							{#each c.players as p (p.index)}
								<BroadcastPlayerCard
									player={p}
									game={data.game}
									teamColor={vm.isTeamGame ? c.color : teamMeta(p.team).color}
									appearance={appearanceFor(p.index)}
									size={118}
									hasTick={vm.hasTick}
									hasScores={vm.hasScores}
								/>
							{/each}
						</div>
					</section>
				{/each}
			</div>
		</div>
	{/if}
</div>

<style>
	.stage {
		position: fixed;
		inset: 0;
		padding: 1.25rem;
		font-family: var(--bc-font);
		color: var(--bc-ink);
		pointer-events: none;
		transform: scale(var(--scale, 1));
		transform-origin: bottom center;
		display: flex;
		align-items: flex-end;
		justify-content: center;
	}

	.status {
		align-self: flex-start;
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

	.strip {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.4rem;
	}
	.badge {
		align-self: center;
		font-size: 0.6rem;
		font-weight: 800;
		letter-spacing: 0.14em;
		background: var(--bc-accent);
		color: #12100a;
		padding: 0.1rem 0.4rem;
		border-radius: 3px;
	}

	.clusters {
		display: flex;
		align-items: flex-end;
		gap: 1.4rem;
		flex-wrap: wrap;
		justify-content: center;
	}
	.cluster {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.cluster-head {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.2rem 0.7rem;
		background: linear-gradient(
			90deg,
			color-mix(in srgb, var(--team) 60%, rgba(6, 8, 12, 0.85)),
			rgba(6, 8, 12, 0.55)
		);
		border-left: 3px solid var(--team);
		border-radius: var(--bc-radius);
	}
	.c-name {
		font-weight: 800;
		font-size: 0.95rem;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.9);
	}
	.c-score {
		margin-left: auto;
		font-size: 1.4rem;
		font-weight: 800;
		font-variant-numeric: tabular-nums;
		text-shadow: 0 1px 4px rgba(0, 0, 0, 0.95);
	}
	.cards {
		display: flex;
		align-items: stretch;
		gap: 0.55rem;
		flex-wrap: wrap;
		justify-content: center;
	}
</style>
