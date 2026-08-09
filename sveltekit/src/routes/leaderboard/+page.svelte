<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked.
	// Rewired to cartographer's native live feed (overlay token + instance),
	// mapped to the pack's match/players shape by overlay-state.
	import { onMount, onDestroy } from 'svelte';
	import { flip } from 'svelte/animate';
	import { quintOut } from 'svelte/easing';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchState, overlayPlayers } from '$lib/utils/overlay-state';
	import { TEAM_HEX } from '$lib/overlay/themes.js';
	import LeaderboardRow from '$lib/overlay/LeaderboardRow.svelte';

	let { data } = $props();
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			console: data.console,
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			consolePoll: data.consolePoll,
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const players = $derived(
		overlayPlayers(feed.game, feed.tick).map((p) => ({ ...p, name: data.names[p.name] ?? p.name }))
	);
	const match = $derived(matchState(feed.game, feed.scenario));

	const teams = $derived(match.teams ?? null);
	const sorted = $derived([...players].sort((a, b) => b.score - a.score));
	const topScore = $derived(sorted[0]?.score ?? 0);
	const topSpree = $derived(Math.max(0, ...players.map((p) => p.spree ?? 0)));
	// Ties share a place number.
	const placeOf = $derived(
		sorted.reduce((m, p, i) => {
			m[p.name] = p.score === sorted[i - 1]?.score ? m[sorted[i - 1].name] : i + 1;
			return m;
		}, {})
	);
	const teamPlayers = (id) => sorted.filter((p) => p.team === id);
</script>

<svelte:head>
	<title>Norcal Halo — leaderboard</title>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Ultra&family=Inter:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<div class="board">
	<div class="head">
		<div class="gm">
			<span class="gt">{match.gametype ?? ''}</span>
			<span class="map">{match.map ?? ''}</span>
		</div>
		<span class="clock">{match.clock ?? '0:00'}</span>
	</div>
	{#if teams}
		{#each teams as team (team.id)}
			<div
				class="teamchip"
				style="background:{TEAM_HEX[team.id]}47; border-color:{TEAM_HEX[team.id]}80"
			>
				<span class="tname">{team.name ?? team.id.toUpperCase() + ' TEAM'}</span>
				<span class="tscore">{team.score}</span>
			</div>
			{#each teamPlayers(team.id) as player (player.name)}
				<div animate:flip={{ duration: 550, easing: quintOut }}>
					<LeaderboardRow
						{player}
						{topSpree}
						topScore={-1}
						forceTint={TEAM_HEX[team.id]}
						avatar={player.avatar}
					/>
				</div>
			{/each}
		{/each}
	{:else}
		{#each sorted as player (player.name)}
			<div animate:flip={{ duration: 550, easing: quintOut }}>
				<LeaderboardRow
					{player}
					{topScore}
					{topSpree}
					place={placeOf[player.name]}
					avatar={player.avatar}
				/>
			</div>
		{/each}
	{/if}
	<div class="footpad"></div>
</div>

<style>
	/* OBS browser source, transparent; size ~340 wide × (72 + 52·rows). */
	:global(html, body) {
		margin: 0;
		background: transparent;
		overflow: hidden;
	}
	/* Kill the app's xbox-theme hex-mesh (body::before) so only the overlay
	   composites over the OBS feed. Unlayered + !important beats the themed
	   @layer base rule in routes/layout.css. */
	:global(body::before) {
		display: none !important;
	}
	.board {
		width: 340px;
		border-radius: 20px;
		overflow: hidden;
		border: 1px solid rgba(159, 180, 208, 0.25);
		box-shadow:
			0 0 26px rgba(61, 98, 224, 0.28),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
		background: rgba(11, 14, 26, 0.95);
		font-family: Inter, sans-serif;
	}
	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 13px 16px 11px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		margin-bottom: 5px;
	}
	.gm {
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	.gt {
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: #9fb4d0;
	}
	.map {
		font-family: Ultra, serif;
		font-size: 17px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.04em;
	}
	.clock {
		font-family: 'Lucida Console', monospace;
		font-size: 24px;
		font-weight: 700;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.teamchip {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin: 4px 8px 3px;
		padding: 8px 12px;
		border-radius: 9px;
		border: 1px solid;
	}
	.tname {
		font-family: Ultra, serif;
		font-size: 15px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.03em;
	}
	.tscore {
		font-family: Ultra, serif;
		font-size: 22px;
		line-height: 1;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.footpad {
		height: 6px;
	}
</style>
