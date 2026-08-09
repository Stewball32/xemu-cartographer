<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked.
	// Rewired to cartographer's native live feed (overlay token + instance),
	// mapped to the pack's match/players shape by overlay-state.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchState, overlayPlayers } from '$lib/utils/overlay-state';
	import { ORANGE, themes } from '$lib/overlay/themes.js';

	let { data } = $props();
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			instance: data.instance,
			token: data.token,
			mock: data.mock,
			classes: ['game', 'tick', 'scenario']
		})
	);
	onDestroy(() => feed.stop());

	const players = $derived(
		overlayPlayers(feed.game, feed.tick).map((p) => ({ ...p, name: data.names[p.name] ?? p.name }))
	);
	const match = $derived(matchState(feed.game, feed.scenario));

	const teams = $derived(match.teams ?? null);
	const red = $derived(teams?.find((t) => t.id === 'red'));
	const blue = $derived(teams?.find((t) => t.id === 'blue'));
	const duel = $derived(
		!teams && players.length === 2 ? [...players].sort((a, b) => b.score - a.score) : null
	);
	const clock = $derived(match.clock ?? '0:00');
	const gametype = $derived(match.gametype ?? '');
	const map = $derived(match.map ?? '');
</script>

<svelte:head>
	<title>Norcal Halo — scorebug</title>
	<link rel="preconnect" href="https://fonts.googleapis.com" />
	<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
	<link
		href="https://fonts.googleapis.com/css2?family=Ultra&family=Inter:wght@400;500;600;700&display=swap"
		rel="stylesheet"
	/>
</svelte:head>

<div class="bug">
	{#if red && blue}
		<div class="panel" style="background:{themes.red.panel}">
			<span class="tscore">{red.score}</span>
			{#key red.score}<span class="flash"></span>{/key}
			<span class="tname" style="color:{themes.red.tagColor}">{red.name ?? 'RED TEAM'}</span>
		</div>
		<div class="center">
			<span class="clock">{clock}</span>
			<span class="gt">{gametype}</span>
			<span class="map">{map}</span>
		</div>
		<div class="panel" style="background:{themes.blue.panel}">
			<span class="tscore">{blue.score}</span>
			{#key blue.score}<span class="flash"></span>{/key}
			<span class="tname" style="color:{themes.blue.tagColor}">{blue.name ?? 'BLUE TEAM'}</span>
		</div>
	{:else if duel}
		<div class="panel dark">
			<span class="pname">{duel[0].name}</span>
			<span class="pscore" style="color:{ORANGE}">{duel[0].score}</span>
		</div>
		<div class="center">
			<span class="clock">{clock}</span>
			<span class="gt">{gametype}</span>
			<span class="map">{map}</span>
		</div>
		<div class="panel dark">
			<span class="pscore">{duel[1].score}</span>
			<span class="pname">{duel[1].name}</span>
		</div>
	{:else}
		<div class="center solo">
			<span class="clock">{clock}</span>
			<span class="gt">{gametype}</span>
			<span class="map">{map}</span>
		</div>
	{/if}
</div>

<style>
	/* OBS browser source, transparent; size the source to fit (~700×90). */
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
	.bug {
		display: inline-flex;
		align-items: stretch;
		height: 84px;
		border-radius: 42px;
		overflow: hidden;
		border: 1px solid rgba(159, 180, 208, 0.25);
		box-shadow:
			0 0 26px rgba(61, 98, 224, 0.28),
			inset 0 1px 0 rgba(255, 255, 255, 0.14);
		font-family: Inter, sans-serif;
	}
	.panel {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 0 30px;
	}
	.panel.dark {
		flex-direction: row;
		gap: 12px;
		background: rgba(11, 14, 26, 0.95);
	}
	.panel + .center,
	.center + .panel {
		border-left: 1px solid rgba(255, 255, 255, 0.1);
	}
	.tscore {
		font-family: Ultra, serif;
		font-size: 32px;
		line-height: 1;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.tname {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.26em;
	}
	.pname {
		font-family: Ultra, serif;
		font-size: 19px;
		line-height: 1;
		color: #e8ecf5;
		letter-spacing: 0.03em;
	}
	.pscore {
		font-size: 30px;
		font-weight: 700;
		line-height: 1;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.center {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 3px;
		padding: 0 22px;
		background: rgba(11, 14, 26, 0.95);
	}
	.center.solo {
		border-radius: 42px;
	}
	.clock {
		font-family: 'Lucida Console', monospace;
		font-size: 24px;
		font-weight: 700;
		color: #e8ecf5;
		font-variant-numeric: tabular-nums;
	}
	.gt {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: #9fb4d0;
	}
	.map {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.26em;
		color: #cdd9ea;
	}
	.flash {
		position: absolute;
		inset: 0;
		pointer-events: none;
		animation: bump 0.7s ease-out;
	}
	@keyframes bump {
		0% {
			background: rgba(255, 176, 65, 0.28);
		}
		100% {
			background: transparent;
		}
	}
</style>
