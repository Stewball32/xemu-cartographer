<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Leaderboard — live standings, FFA (ranked flat list) or team (two chipped
	// blocks). Ported from the obs-handoff pack's leaderboard.html, wired to
	// cartographer's native live feed via overlay-state.
	//
	// OBS browser source: 340 wide; height scales with the roster at ~52px per
	// row plus the header (≈330 for a 5-player FFA, ≈560 for a 4v4). Size the
	// source generously and let the transparent area fall where it may.
	//
	// Rows are absolutely positioned and animate their `top`, so a re-sort slides
	// instead of snapping. Keyed by player name to hold identity across frames.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchState, overlayPlayers, rankPlayers } from '$lib/utils/overlay-state';
	import LeaderboardRow from '$lib/overlay/LeaderboardRow.svelte';
	import starUrl from '$lib/assets/star.png';
	import '$lib/styles/overlay-base.css';

	// Row pitch: 46px card + 3px padding top and bottom. Must match the slot
	// padding in .rowslot below and the row height in LeaderboardRow.
	const ROW_PITCH = 52;

	let { data } = $props();
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			console: data.console,
			mock: data.mock,
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const players = $derived(
		overlayPlayers(feed.game, feed.tick).map((p) => ({ ...p, name: data.names[p.name] ?? p.name }))
	);
	const match = $derived(matchState(feed.game, feed.scenario));
	const isTeam = $derived(match.mode === 'team');

	const red = $derived(match.teams?.find((t) => t.id === 'red'));
	const blue = $derived(match.teams?.find((t) => t.id === 'blue'));

	const ffaRanked = $derived(isTeam ? [] : rankPlayers(players));
	const redRanked = $derived(isTeam ? rankPlayers(players.filter((p) => p.team === 'red')) : []);
	const blueRanked = $derived(isTeam ? rankPlayers(players.filter((p) => p.team === 'blue')) : []);
</script>

<svelte:head>
	<title>NorCal Halo — leaderboard</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="board">
		<div class="head" class:has-teams={isTeam} style="--emblem:url({starUrl})">
			<div class="head-id">
				<span class="gametype">{match.gametype}</span>
				<span class="map">{match.map}</span>
			</div>
			<span class="clock">{match.clock ?? '0:00'}</span>
		</div>

		{#if isTeam}
			{#if red}
				<div class="teamchip is-red">
					<div>
						<span class="teamchip-name">{red.name}</span>
						<span class="teamchip-score">{red.score}</span>
					</div>
				</div>
			{/if}
			<div class="rows" style="height:{redRanked.length * ROW_PITCH}px">
				{#each redRanked as p, i (p.name)}
					<div class="rowslot" style="top:{i * ROW_PITCH}px">
						<LeaderboardRow player={p} />
					</div>
				{/each}
			</div>

			{#if blue}
				<div class="teamchip is-blue">
					<div>
						<span class="teamchip-name">{blue.name}</span>
						<span class="teamchip-score">{blue.score}</span>
					</div>
				</div>
			{/if}
			<div class="rows" style="height:{blueRanked.length * ROW_PITCH}px">
				{#each blueRanked as p, i (p.name)}
					<div class="rowslot" style="top:{i * ROW_PITCH}px">
						<LeaderboardRow player={p} />
					</div>
				{/each}
			</div>
		{:else}
			<div class="rows" style="height:{ffaRanked.length * ROW_PITCH}px">
				{#each ffaRanked as p, i (p.name)}
					<div class="rowslot" style="top:{i * ROW_PITCH}px">
						<LeaderboardRow player={p} rank={i + 1} leader={i === 0} />
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<style>
	/* Transparent canvas — see the scorebug for why both html and body are reset
	   and why body::before/::after are killed. */
	:global(html, body) {
		margin: 0;
		padding: 0;
		background: transparent !important;
		background-image: none !important;
		overflow: hidden;
	}
	:global(body::before, body::after) {
		display: none !important;
		content: none !important;
	}

	.stage[data-anchor='center'] {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100vw;
		height: 100vh;
	}

	.board {
		width: 340px;
		border-radius: 12px;
		overflow: hidden;
		border: var(--nh-edge);
		box-shadow: var(--nh-lift);
		background: var(--nh-panel);
		padding-bottom: 5px;
		font-family: Inter, system-ui, sans-serif;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 13px 16px 11px;
		border-bottom: var(--nh-hairline);
		background:
			linear-gradient(rgba(11, 14, 26, 0.66), rgba(11, 14, 26, 0.66)),
			var(--emblem) center / 165px no-repeat;
	}
	.head.has-teams {
		margin-bottom: 5px;
	}
	.head-id {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.gametype {
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: var(--nh-steel);
	}
	.map {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 14px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.06em;
	}
	.clock {
		font-family: 'Lucida Console', monospace;
		font-size: 24px;
		font-weight: 700;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}

	.rows {
		position: relative;
	}
	.rowslot {
		position: absolute;
		left: 0;
		right: 0;
		padding: 3px 8px;
		transition: top 0.55s cubic-bezier(0.22, 1, 0.36, 1);
	}

	.teamchip {
		padding: 3px 8px;
	}
	.teamchip.is-blue {
		margin-top: 4px;
	}
	.teamchip > div {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 8px 12px;
		border-radius: 6px;
	}
	.teamchip.is-red > div {
		background: rgba(224, 82, 82, 0.28);
		border: 1px solid rgba(224, 82, 82, 0.5);
	}
	.teamchip.is-blue > div {
		background: rgba(61, 98, 224, 0.28);
		border: 1px solid rgba(61, 98, 224, 0.5);
	}
	.teamchip-name {
		font-family: Orbitron, sans-serif;
		font-weight: 700;
		font-size: 12px;
		line-height: 1;
		color: var(--nh-text);
		letter-spacing: 0.04em;
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.teamchip-score {
		font-family: Orbitron, sans-serif;
		font-weight: 800;
		font-size: 20px;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}

	@media (prefers-reduced-motion: reduce) {
		.rowslot {
			transition: none;
		}
	}
</style>
