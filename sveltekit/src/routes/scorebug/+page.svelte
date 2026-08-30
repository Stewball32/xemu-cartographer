<script>
	// @ts-nocheck — OBS overlay graphic (plain JS); not strict-TS checked.
	//
	// Scorebug — redesign (CL-01/06/09/12/18). Fixed 216px sides keep the match
	// center dead-centered; Inter numerals; the leading score takes Selection
	// Orange; kill pop on score change. Team names wrap to two lines at 11px.
	// FFA: 3+ players → top-FOUR podium on mottoless nameplates (1st leads left,
	// 2nd–4th stack right); exactly two keeps the head-to-head duel on plates.
	// Motion 1b: CRT power-on (scanline snap, 0.5s) on source activate; CRT
	// power-off (collapse → line → dot, 0.42s) at match end.
	// NOTE: this is its own browser source, independent of /overlay — no
	// wiring between them. The POV bars bake in a 120ms head delay so the
	// bug reads first when one scene switch activates both.
	//
	// OBS browser source: ~700×84, transparent; graphic renders at natural size
	// top-left. Add ?anchor=center to centre it in a larger box.
	import { onMount, onDestroy } from 'svelte';
	import { untrack } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import {
		applyIdentities,
		matchState,
		overlayPlayers,
		rankPlayers
	} from '$lib/utils/overlay-state';
	import { createProfileLookup } from '$lib/stores/overlay-profiles.svelte';
	import NamePlate from '$lib/overlay/NamePlate.svelte';
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

	const lookup = createProfileLookup();
	const scraped = $derived(overlayPlayers(feed.game, feed.tick));
	$effect(() => {
		lookup.ensure(
			scraped.map((p) => p.name),
			data.mock
		);
	});
	const players = $derived(applyIdentities(scraped, lookup.all, data.names));
	const match = $derived(matchState(feed.game, feed.scenario));

	// Match-end latch. `game.over` is an optional carnage-report flag the
	// scraper does not currently send, so deriving the out purely from it meant
	// the power-off never played. Latch instead on this source having SEEN a
	// live match: once it has, the match leaving `live` is the end of it.
	//
	// Ordering matters — a source activated after a match already ended never
	// sees `live`, so it stays visible rather than powering off on arrival. A
	// dropped feed (game null) is not an ending, so a reconnect blip doesn't
	// fire it either. Re-entering `live` clears the latch, so the source comes
	// back for the next match without a remount.
	let sawLive = $state(false);
	$effect(() => {
		const live = feed.game?.phase === 'live';
		untrack(() => {
			if (live) sawLive = true;
		});
	});
	const over = $derived(
		!!feed.game?.over || (sawLive && !!feed.game && feed.game.phase !== 'live')
	);

	const teams = $derived(match.teams ?? null);
	const red = $derived(teams?.find((t) => t.id === 'red'));
	const blue = $derived(teams?.find((t) => t.id === 'blue'));

	// FFA: podium at 3+, duel at exactly 2 (CL-09).
	const ranked = $derived(match.mode === 'team' ? [] : rankPlayers(players));
	const podium = $derived(match.mode !== 'team' && ranked.length >= 3 ? ranked.slice(0, 4) : null);
	const duel = $derived(match.mode !== 'team' && !podium ? ranked.slice(0, 2) : null);
	const tied = $derived(!!duel && duel.length === 2 && duel[0].score === duel[1].score);

	// Kill pop on score change (CL-06).
	let redPop = $state(false);
	let bluePop = $state(false);
	let prevRed, prevBlue, rt, bt;
	$effect(() => {
		const s = red?.score;
		untrack(() => {
			if (prevRed !== undefined && s > prevRed) {
				redPop = false;
				clearTimeout(rt);
				requestAnimationFrame(() => (redPop = true));
				rt = setTimeout(() => (redPop = false), 750);
			}
			prevRed = s;
		});
	});
	$effect(() => {
		const s = blue?.score;
		untrack(() => {
			if (prevBlue !== undefined && s > prevBlue) {
				bluePop = false;
				clearTimeout(bt);
				requestAnimationFrame(() => (bluePop = true));
				bt = setTimeout(() => (bluePop = false), 750);
			}
			prevBlue = s;
		});
	});
</script>

<svelte:head>
	<title>NorCal Halo — scorebug</title>
</svelte:head>

<div class="stage" data-anchor={data.anchor}>
	<div class="scorebug" class:out={over} style="--emblem:url({starUrl})">
		{#if red && blue}
			<div class="team is-red">
				<span class="score" class:is-leader={red.score > blue.score} class:pop={redPop}
					>{red.score}</span
				>
				<span class="is-red label">{red.name}</span>
			</div>
		{:else if podium}
			<div class="team is-ffa-left pd-lead">
				<span class="score is-leader">{podium[0].score}</span>
				<NamePlate player={podium[0]} h={32} showMotto={false} />
			</div>
		{:else if duel}
			<div class="team is-ffa-left pd-lead">
				<span class="score" class:is-leader={!tied}>{duel[0]?.score ?? 0}</span>
				{#if duel[0]}<NamePlate player={duel[0]} h={26} showMotto={false} />{/if}
			</div>
		{/if}

		<div class="centre">
			<span class="clock">{match.clock ?? '0:00'}</span>
			<span class="gametype">{match.gametype}</span>
			<span class="map">{match.map}</span>
		</div>

		{#if red && blue}
			<div class="team is-blue">
				<span class="score" class:is-leader={blue.score > red.score} class:pop={bluePop}
					>{blue.score}</span
				>
				<span class="is-blue label">{blue.name}</span>
			</div>
		{:else if podium}
			<div class="team is-ffa-right pd-stack">
				{#each podium.slice(1) as p (p.name)}
					<div class="pd-row">
						<span class="pd-score">{p.score}</span>
						<NamePlate player={p} h={22} showMotto={false} />
					</div>
				{/each}
			</div>
		{:else if duel}
			<div class="team is-ffa-right pd-lead">
				<span class="score">{duel[1]?.score ?? 0}</span>
				{#if duel[1]}<NamePlate player={duel[1]} h={26} showMotto={false} />{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	/* Transparent canvas — html + body reset, themed pseudo-elements killed;
	   unlayered + !important beats the themed @layer base rules. */
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
		transform-origin: 50% 50%;
		animation: crt-in 0.5s ease-out both;
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

	/* Fixed, equal sides — the centre block never drifts. */
	.team {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 5px;
		width: 216px;
		box-sizing: border-box;
		padding: 0 14px;
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
	.pd-lead {
		gap: 4px;
		padding: 0 12px;
	}

	/* Inter numerals suite-wide (CL-12); orange only while leading (CL-01). */
	.score {
		font-family: Inter, system-ui, sans-serif;
		font-weight: 800;
		font-size: 29px;
		line-height: 1;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
	}
	.score.is-leader {
		color: var(--nh-orange);
	}
	.score.pop {
		display: inline-block;
		animation: killpop 0.7s ease-out both;
	}

	.label {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.26em;
		max-width: 200px;
		white-space: normal;
		text-align: center;
		line-height: 1.35;
	}
	.label.is-red {
		color: #ff8f85;
	}
	.label.is-blue {
		color: #7d9cff;
	}

	/* 2nd–4th stack — rank implied by order. */
	.pd-stack {
		gap: 4px;
		padding: 0 12px;
		justify-content: center;
	}
	.pd-row {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.pd-score {
		font-family: Inter, system-ui, sans-serif;
		font-weight: 800;
		font-size: 14px;
		color: var(--nh-text);
		font-variant-numeric: tabular-nums;
		flex: none;
	}

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

	.scorebug.out {
		animation: crt-out 0.42s ease-in both;
	}
	@keyframes crt-in {
		0% {
			opacity: 0;
			transform: scaleY(0.008) scaleX(0.55);
			filter: brightness(7) blur(1px);
		}
		10% {
			opacity: 1;
		}
		42% {
			transform: scaleY(0.014) scaleX(1.02);
			filter: brightness(4.5) blur(0.5px);
		}
		74% {
			transform: scaleY(1.05) scaleX(1);
			filter: brightness(1.7);
		}
		100% {
			opacity: 1;
			transform: none;
			filter: brightness(1);
		}
	}
	@keyframes crt-out {
		0% {
			opacity: 1;
			transform: none;
			filter: brightness(1);
		}
		45% {
			opacity: 1;
			transform: scaleY(0.012) scaleX(1.03);
			filter: brightness(6);
		}
		72% {
			transform: scaleY(0.012) scaleX(0.16);
			filter: brightness(9);
		}
		100% {
			opacity: 0;
			transform: scaleY(0.01) scaleX(0.002);
			filter: brightness(10);
		}
	}
	@keyframes killpop {
		0% {
			transform: scale(1.55);
			color: #ffb041;
			text-shadow: 0 0 14px rgba(255, 176, 65, 0.8);
		}
		38% {
			transform: scale(1);
			text-shadow: none;
		}
		100% {
			transform: scale(1);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.score.pop,
		.scorebug,
		.scorebug.out {
			animation: none;
		}
	}
</style>
