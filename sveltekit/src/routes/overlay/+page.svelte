<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked.
	// Rewired to cartographer's live feed: splitscreen is AUTO-detected server-
	// side (overlay-split), NOT a manual OBS/URL toggle. The layout re-derives
	// reactively, so the overlay re-lays-out live when the split changes.
	import { onMount, onDestroy } from 'svelte';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { deriveSplitCount, layoutKey, localOverlayPlayers } from '$lib/utils/overlay-split';
	import { ordinal, overlayPlayers, rankPlayers } from '$lib/utils/overlay-state';
	import { layouts, viewportCenters } from '$lib/overlay/themes.js';
	import PlayerCard from '$lib/overlay/PlayerCard.svelte';
	import RespawnRing from '$lib/overlay/RespawnRing.svelte';
	import '$lib/styles/overlay-base.css';

	let { data } = $props();

	// One subscription to THIS instance's live game + tick classes (or mock).
	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			console: data.console,
			mock: data.mock,
			classes: ['game', 'tick']
		})
	);
	onDestroy(() => feed.stop());

	// Player selection:
	//  • console mode (?console=NAME): show that ONE console's own seat(s),
	//    selected by the resolver's live machine index (indices shift as the
	//    lobby changes, so this re-selects every snapshot — BlueBox always shows
	//    BlueBox, never RedBox).
	//  • instance mode: the host's local splitscreen, auto-detected.
	const consoleMode = $derived(!!data.console);
	const rawPlayers = $derived(
		consoleMode
			? overlayPlayers(feed.game, feed.tick, feed.machineIndex)
			: localOverlayPlayers(feed.game, feed.tick)
	);
	const split = $derived(
		data.layoutOverride ||
			(consoleMode ? Math.max(rawPlayers.length, 1) : deriveSplitCount(feed.game, feed.tick))
	);
	const key = $derived(layoutKey(split));
	const anchors = $derived(layouts[key]);
	const centers = $derived(viewportCenters[key]);
	const players = $derived(
		rawPlayers.slice(0, anchors.length).map((p) => ({ ...p, name: data.names[p.name] ?? p.name }))
	);

	// Placing is ranked across the WHOLE lobby, not just the seats this source
	// shows — a split-screen box's 2nd-place player is 2nd in the match, not 2nd
	// of the two on screen. Names carry the same ?names= override as the cards so
	// the lookup below matches.
	const lobbyOrder = $derived(
		rankPlayers(overlayPlayers(feed.game, feed.tick)).map((p) => data.names[p.name] ?? p.name)
	);
	const placeOf = (name) => {
		const i = lobbyOrder.indexOf(name);
		return i < 0 ? '' : ordinal(i);
	};

	const pos = (a) =>
		`left:${a.left ?? 'auto'}; top:${a.top ?? 'auto'}; bottom:${a.bottom ?? 'auto'}; transform:${a.tf ?? 'none'};`;
</script>

<svelte:head>
	<title>NorCal Halo — POV overlay</title>
</svelte:head>

<div class="canvas">
	{#each players as player, i (player.slot ?? i)}
		<div class="anchor" style={pos(anchors[i])}>
			<PlayerCard
				{player}
				scale={anchors[i].scale ?? 1}
				origin={anchors[i].origin ?? 'center'}
				place={placeOf(player.name)}
				sheen="{(i * 1.3).toFixed(1)}s"
			/>
		</div>
		{#if player.alive === false && (player.respawn ?? 0) > 0}
			<div class="ring-anchor" style="left:{centers[i].x}px; top:{centers[i].y}px">
				<RespawnRing
					seconds={player.respawn}
					max={player.respawnMax ?? 5}
					theme={player.team ?? 'ffa'}
				/>
			</div>
		{/if}
	{/each}
</div>

<style>
	/* OBS browser source: 1440x1080 (4:3, matching the Xbox/CE player view),
	   transparent. Skeleton v5 paints the root background on `html` (v4 used
	   `body`), so BOTH must be neutralised — and body::before/::after kill the
	   themed decorations (xbox's hex mesh, starcommand's vignette) that would
	   otherwise bake into the capture. Unlayered + !important beats the themed
	   @layer base rules in routes/layout.css. */
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
	.canvas {
		position: relative;
		width: 1440px;
		height: 1080px;
	}
	.anchor {
		position: absolute;
		width: 700px;
		height: 64px;
	}
	.ring-anchor {
		position: absolute;
		transform: translate(-50%, -50%);
	}
</style>
