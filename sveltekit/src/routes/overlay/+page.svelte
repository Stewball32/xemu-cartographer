<script>
	// @ts-nocheck — vendored OBS overlay pack (plain JS); not strict-TS checked.
	// POV overlay — redesign: 820×72 plate bars, lobby-best score orange,
	// KILLED BY respawn rings. Splitscreen is AUTO-detected server-side
	// (overlay-split); the layout re-derives reactively.
	// Motion 1a: bars deploy up from the bottom rail on activate, staggered
	// 120ms in seat order; the leading 120ms is headroom so the scorebug (a
	// separate browser source) reads first when one scene switch shows both.
	// All drop away together on `game.over`. Motion 2a: the respawn ring's disc locks on every death
	// (in RespawnRing.svelte — the plate rises in behind it); this page owns
	// only the ring's EXIT via the out: transition below.
	import { onMount, onDestroy } from 'svelte';
	import { cubicIn } from 'svelte/easing';
	import { createOverlayFeed } from '$lib/stores/overlay-feed.svelte';
	import { deriveSplitCount, layoutKey, localOverlayPlayers } from '$lib/utils/overlay-split';
	import { applyIdentities, overlayPlayers } from '$lib/utils/overlay-state';
	import { createProfileLookup } from '$lib/stores/overlay-profiles.svelte';
	import { layouts, viewportCenters } from '$lib/overlay/themes.js';
	import PlayerCard from '$lib/overlay/PlayerCard.svelte';
	import RespawnRing from '$lib/overlay/RespawnRing.svelte';
	import '$lib/styles/overlay-base.css';

	let { data } = $props();

	const feed = createOverlayFeed();
	onMount(() =>
		feed.start({
			console: data.console,
			mock: data.mock,
			classes: ['game', 'tick']
		})
	);
	onDestroy(() => feed.stop());

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

	// The whole lobby is resolved (not just this source's seats) so every
	// surface agrees on handles, mottos and the lobby-best score.
	const lookup = createProfileLookup();
	const lobby = $derived(overlayPlayers(feed.game, feed.tick));
	$effect(() => {
		lookup.ensure(
			lobby.map((p) => p.name),
			data.mock
		);
	});
	const players = $derived(
		applyIdentities(rawPlayers.slice(0, anchors.length), lookup.all, data.names)
	);
	const lobbyIdentified = $derived(applyIdentities(lobby, lookup.all, data.names));

	// Lobby-best score → Selection Orange on the bar (CL-01). Ties: all leaders.
	const topScore = $derived(Math.max(0, ...lobbyIdentified.map((p) => p.score ?? 0)));

	// Killer identity for the respawn ring (CL-18) — resolved against the lobby
	// so the plate carries the killer's display name, avatar and motto fields.
	const killerOf = (p) =>
		p.killedBy ? (lobbyIdentified.find((q) => q.name === p.killedBy) ?? null) : null;

	const pos = (a) =>
		`left:${a.left ?? 'auto'}; top:${a.top ?? 'auto'}; bottom:${a.bottom ?? 'auto'}; transform:${a.tf ?? 'none'};`;

	// Optional carnage-report flag (see README) — absent, no out plays and the
	// source just hides on scene switch.
	const over = $derived(!!feed.game?.over);

	// Ring release (motion 2a out) — entry lives in RespawnRing.svelte (it
	// replays each death since the {#if} remounts); removal can only be
	// animated from here, so the page owns exit like it owns placement.
	const rm =
		typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;
	const ringOut = () => ({
		duration: rm ? 0 : 260,
		easing: cubicIn,
		css: (t, u) =>
			`opacity:${t}; transform:scale(${1 - 0.14 * u}); filter:brightness(${1 + 0.8 * u})`
	});
</script>

<svelte:head>
	<title>NorCal Halo — POV overlay</title>
</svelte:head>

<div class="canvas">
	{#each players as player, i (player.slot ?? i)}
		<div class="anchor" style={pos(anchors[i])}>
			<!-- inner wrapper animates so .anchor's positioning transform stays put -->
			<div class="deploy" class:out={over} style="--in-delay:{120 + i * 120}ms">
				<PlayerCard
					{player}
					scale={anchors[i].scale ?? 1}
					origin={anchors[i].origin ?? 'center'}
					leader={topScore > 0 && (player.score ?? 0) === topScore}
					sheen="{(i * 1.3).toFixed(1)}s"
				/>
			</div>
		</div>
		{#if player.alive === false && (player.respawn ?? 0) > 0}
			<div class="ring-anchor" style="left:{centers[i].x}px; top:{centers[i].y}px">
				<!-- seconds is respawn_in_ticks ÷ 30 UNROUNDED — the disc drains
				     continuously (CL-17); max is the gametype respawn time (CL-11). -->
				<div out:ringOut>
					<RespawnRing
						seconds={player.respawn}
						max={player.respawnMax ?? 8}
						theme={player.team ?? 'ffa'}
						killer={killerOf(player)}
					/>
				</div>
			</div>
		{/if}
	{/each}
</div>

<style>
	/* OBS browser source: 1440x1080 (4:3), transparent — see the scorebug for
	   why html + body are reset and body::before/::after are killed. */
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
		width: 820px;
		height: 72px;
	}
	.ring-anchor {
		position: absolute;
		transform: translate(-50%, -50%);
	}
	/* Motion 1a — deploy slide from the bottom rail, staggered per seat. */
	.deploy {
		animation: pov-in 0.55s cubic-bezier(0.22, 1, 0.36, 1) var(--in-delay, 0ms) both;
	}
	.deploy.out {
		animation: pov-out 0.32s cubic-bezier(0.5, 0, 0.75, 0.4) both;
	}
	@keyframes pov-in {
		0% {
			opacity: 0;
			transform: translateY(32px);
		}
		100% {
			opacity: 1;
			transform: none;
		}
	}
	@keyframes pov-out {
		0% {
			opacity: 1;
			transform: none;
		}
		100% {
			opacity: 0;
			transform: translateY(26px);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.deploy,
		.deploy.out {
			animation: none;
		}
	}
</style>
