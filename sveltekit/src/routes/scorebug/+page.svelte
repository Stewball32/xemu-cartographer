<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Scorebug — match state: clock, gametype, map, both scores.
	// Ported from the obs-handoff pack's scorebug.html, wired to cartographer's
	// native live feed (game / tick / scenario classes) via overlay-state.
	//
	// OBS browser source: 320×84 (FFA duel) / 480×84 (team, long names). The
	// graphic renders at its natural size at the top-left of the source; add
	// ?anchor=center to centre it in a larger box instead.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { matchState, overlayPlayers, rankPlayers } from '$lib/utils/overlay-state';
	import starUrl from '$lib/assets/star.png';
	import '$lib/styles/overlay-base.css';

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

	const teams = $derived(match.teams ?? null);
	const red = $derived(teams?.find((t) => t.id === 'red'));
	const blue = $derived(teams?.find((t) => t.id === 'blue'));

	// FFA duel — the top two by rank. The leader's score takes Selection Orange,
	// but only when someone is actually ahead: a tie highlights neither.
	const duel = $derived(match.mode === 'team' ? null : rankPlayers(players).slice(0, 2));
	const tied = $derived(!!duel && duel.length === 2 && duel[0].score === duel[1].score);
</script>

<svelte:head>
	<title>NorCal Halo — scorebug</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="scorebug" style="--emblem:url({starUrl})">
		{#if red && blue}
			<div class="team is-red">
				<span class="score">{red.score}</span>
				<span class="is-red label">{red.name}</span>
			</div>
		{:else if duel}
			<div class="team is-ffa-left">
				<span class="score" class:is-leader={!tied}>{duel[0]?.score ?? 0}</span>
				<span class="is-ffa label">{duel[0]?.name ?? '—'}</span>
			</div>
		{/if}

		<div class="centre">
			<span class="clock">{match.clock ?? '0:00'}</span>
			<span class="gametype">{match.gametype}</span>
			<span class="map">{match.map}</span>
		</div>

		{#if red && blue}
			<div class="team is-blue">
				<span class="score">{blue.score}</span>
				<span class="is-blue label">{blue.name}</span>
			</div>
		{:else if duel}
			<div class="team is-ffa-right">
				<span class="score">{duel[1]?.score ?? 0}</span>
				<span class="is-ffa label">{duel[1]?.name ?? '—'}</span>
			</div>
		{/if}
	</div>
</div>

<style>
	/* Transparent canvas: OBS composites this over the game feed. Skeleton v5
	   paints the root background on `html` (v4 used `body`), so BOTH must be
	   neutralised — and `body::before` kills the xbox theme's hex mesh, which
	   would otherwise bake into the capture. Unlayered + !important beats the
	   themed @layer base rules in routes/layout.css. */
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

	.scorebug {
		display: flex;
		align-items: stretch;
		height: 84px;
		width: max-content;
		border-radius: 12px;
		overflow: hidden;
		border: var(--nh-edge);
		box-shadow: var(--nh-lift);
		font-family: Inter, system-ui, sans-serif;
	}

	.team {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 5px;
		padding: 0 30px;
		background: var(--nh-panel);
	}
	.team.is-red {
		background: rgba(44, 11, 11, 0.95);
		border-right: var(--nh-hairline);
	}
	.team.is-blue {
		background: rgba(10, 16, 44, 0.95);
		border-left: var(--nh-hairline);
	}
	.team.is-ffa-left {
		border-right: var(--nh-hairline);
	}
	.team.is-ffa-right {
		border-left: var(--nh-hairline);
	}

	.score {
		font-family: Orbitron, sans-serif;
		font-weight: 800;
		font-size: 29px;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.score.is-leader {
		color: var(--nh-orange);
	}

	.label {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.26em;
		max-width: 15ch;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.label.is-red {
		color: #ff8f85;
	}
	.label.is-blue {
		color: #7d9cff;
	}
	.label.is-ffa {
		color: #5d82ff;
	}

	/* Centre panel — the emblem reads through at 220px / 46%, with the scrim
	   matched to the leaderboard and post-game headers (0.66). */
	.centre {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 3px;
		padding: 0 22px;
		background:
			linear-gradient(rgba(11, 14, 26, 0.66), rgba(11, 14, 26, 0.66)),
			var(--emblem) center 46% / 220px no-repeat,
			var(--nh-panel);
	}
	.clock {
		font-family: 'Lucida Console', monospace;
		font-size: 24px;
		font-weight: 700;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
		text-shadow:
			0 1px 3px rgba(0, 0, 0, 0.9),
			0 0 10px rgba(11, 14, 26, 0.85);
	}
	/* Lifted off the base palette so 9px type survives the emblem's muzzle and
	   rifle highlights at stream resolution. Do not darken these. */
	.gametype,
	.map {
		font-size: 9px;
		font-weight: 700;
		letter-spacing: 0.26em;
		text-shadow:
			0 1px 3px rgba(0, 0, 0, 0.95),
			0 0 8px rgba(11, 14, 26, 0.9);
	}
	.gametype {
		color: #dce6f2;
	}
	.map {
		color: #eef4fb;
	}
</style>
